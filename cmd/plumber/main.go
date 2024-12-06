// Command plumber implements a daemon for interprocess messaging.
package main

import (
	"bufio"
	"bytes"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"syscall"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/thimc/plumber/internal"
)

var (
	logfile   = flag.String("l", "log", "log file")
	sendfile  = flag.String("p", "send", "send file")
	rulesfile = flag.String("r", "rules", "rules file")
)

type VerbType int

const (
	VerbIs VerbType = iota
	VerbIsNot
	VerbIsDir
	VerbIsFile
	VerbMatches
	VerbSet
	VerbStart
	VerbTo
)

var verbs = map[string]VerbType{
	"is":      0,
	"isn't":   1,
	"isdir":   2,
	"isfile":  3,
	"matches": 4,
	"set":     5,
	"start":   6,
	"to":      7,
}

func (v VerbType) String() string {
	for k, vt := range verbs {
		if v == vt {
			return k
		}
	}
	return "?"
}

type Ruleset struct {
	Patterns  []Pattern
	Variables map[string]string
}

func (r *Ruleset) Evaluate() error {
	for _, p := range r.Patterns {
		err := p.Evaluate(r)
		if err != nil {
			return err
		}
	}
	return nil
}

var errNoMatch = fmt.Errorf("no match")

type Pattern struct {
	Object string   // The object is the field to be matched
	Verb   VerbType // Verb describes how the comparison should be done
	Arg    string   // Arg is the data the Object should be compared with
}

func (p *Pattern) Evaluate(r *Ruleset) (err error) {
	if !slices.Contains([]string{"arg", "data", "dst", "plumb", "src", "type", "wdir"}, p.Object) {
		return fmt.Errorf("invalid object: %+v", p)
	}
	p.Arg, err = r.Expand(p.Arg)
	if err != nil {
		return err
	}

	if p.Object == "plumb" {
		switch p.Verb {
		case VerbStart:
			cmd := exec.Command(internal.DefaultShell, internal.DefaultShellArg, p.Arg)
			cmd.Env = os.Environ()
			cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
			cmd.Dir = r.Variables["wdir"]
			if err = cmd.Start(); err != nil {
				return err
			}
			log.Println(cmd)
			return cmd.Process.Release()
		case VerbTo:
			f, err := os.OpenFile(p.Arg, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0644)
			if err != nil {
				return err
			}
			defer f.Close()
			data, ok := r.Variables["arg"]
			if !ok {
				return fmt.Errorf("missing data: %+v", r)
			}
			if _, err = f.WriteString(data); err != nil {
				return err
			}
			return nil
		default:
			return fmt.Errorf("unexpected %s verb: %+v", p.Object, p)
		}
	}
	switch p.Verb {
	case VerbIs:
		if p.Arg == r.Variables[p.Object] {
			return nil
		}
	case VerbIsNot:
		if p.Arg != r.Variables[p.Object] {
			return nil
		}
	case VerbIsDir, VerbIsFile:
		if !filepath.IsAbs(p.Arg) {
			wdir, ok := r.Variables["wdir"]
			if !ok {
				log.Printf("missing working directory: %q", p.Arg)
				return errNoMatch
			}
			p.Arg = filepath.Join(wdir, p.Arg)
		}
		p.Arg = filepath.Clean(p.Arg)
		fi, err := os.Stat(p.Arg)
		if err != nil {
			return err
		}
		if p.Verb == VerbIsDir && fi.Mode().IsDir() {
			r.Variables["dir"] = p.Arg
			return nil
		} else if p.Verb == VerbIsFile && fi.Mode().IsRegular() {
			r.Variables["file"] = p.Arg
			return nil
		}
	case VerbMatches:
		re, err := regexp.Compile(p.Arg)
		if err != nil {
			return err
		}
		if re.MatchString(r.Variables[p.Object]) {
			for i, subm := range re.FindStringSubmatch(r.Variables[p.Object]) {
				r.Variables[fmt.Sprint(i)] = subm
			}
			return nil
		}
	case VerbSet:
		r.Variables[p.Object] = p.Arg
		return nil
	default:
		return fmt.Errorf("unexpected %s verb: %+v", p.Object, p)
	}
	return errNoMatch
}

func (r *Ruleset) Expand(s string) (string, error) {
	var (
		quoted   bool
		escaped  bool
		variable bool
		sb       strings.Builder
		buf      string
		count    = utf8.RuneCountInString(s)
	)
	for i := 0; i < count; {
		ch, siz := utf8.DecodeRuneInString(s[i:])
		switch ch {
		case '\'':
			if !escaped {
				quoted = !quoted
				i += siz
				continue
			}
		case '\\':
			if !quoted {
				escaped = true
				i += siz
				continue
			}
		}
		if !escaped && !quoted && ch == '$' {
			i += siz
			ch, siz = utf8.DecodeRuneInString(s[i:])
			if ch == '{' {
				i += siz
				variable = true
			}
			var j int
			for j = i; j < count; {
				ch, siz := utf8.DecodeRuneInString(s[j:])
				if !unicode.IsLetter(ch) && !unicode.IsNumber(ch) {
					if variable && ch == '}' {
						variable = false
					}
					break
				}
				buf += string(ch)
				j += siz
			}
			if j >= count && variable {
				return "", fmt.Errorf("expected closing brace: %q", s)
			}
			i = j
			data, ok := r.Variables[buf]
			if !ok {
				return "", fmt.Errorf("unknown variable: %q", buf)
			}
			sb.WriteString(data)
			buf = ""
			continue
		}
		sb.WriteRune(ch)
		escaped = false
		i += siz
	}
	if quoted {
		return "", fmt.Errorf("expected closing quote: %q", s)
	}
	return sb.String(), nil
}

func main() {
	internal.Defaults()
	flag.Parse()
	logf, err := os.OpenFile(*logfile, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		log.Println(err)
	} else {
		defer logf.Close()
		log.SetOutput(logf)
	}
	if _, err := os.Stat(*sendfile); err == os.ErrNotExist {
		if err := syscall.Mkfifo(*sendfile, 0644); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	}
	send, err := os.OpenFile(*sendfile, os.O_RDONLY, os.ModeNamedPipe)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	defer send.Close()
	for {
		var b bytes.Buffer
		n, err := io.Copy(&b, send)
		if err != nil {
			log.Println("copy:", err)
			continue
		}
		if n == 0 {
			time.Sleep(100 * time.Millisecond)
			continue
		}
		var msg internal.Message
		if err := msg.Decode(&b); err != nil {
			log.Printf("decode %+v: %q", string(b.Bytes()), err)
			continue
		}
		log.Printf("received %s", msg.String())
		go process(msg, *rulesfile)
	}

}

func process(msg internal.Message, rulefile string) {
	base := Ruleset{
		Variables: map[string]string{
			"data": string(msg.Data),
			"dst":  msg.Dst,
			"src":  msg.Src,
			"type": msg.Type,
			"wdir": msg.Wdir,
			"file": "",
			"dir":  "",
		}}
	f, err := os.Open(rulefile)
	if err != nil {
		log.Println(err)
		return
	}
	defer f.Close()
	var (
		rule    = base
		s       = bufio.NewScanner(f)
		capture bool
		p       Pattern
	)
	for s.Scan() || capture {
		line := s.Text()
		if len(line) == 0 {
			if len(rule.Patterns) == 0 {
				continue
			}
			if err := rule.Evaluate(); err != nil {
				if err != errNoMatch {
					log.Println(err)
					return
				}
				capture = !capture
				rule = base
				continue
			}
			return
		} else if strings.HasPrefix(line, "#") {
			continue
		} else if strings.Contains(line, "=") {
			parts := strings.Split(line, "=")
			if len(parts) < 1 {
				log.Printf("invalid assignment: %q", line)
				continue
			}
			name := strings.Trim(parts[0], " \t")
			value, err := rule.Expand(strings.Trim(strings.Join(parts[1:], "="), " \t"))
			if err != nil {
				log.Println(err)
				continue
			}
			rule.Variables[name] = value
			log.Printf("%q = %q", name, value)
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 2 {
			log.Printf("malformed rule: %q", line)
			return
		}
		verb, ok := verbs[fields[1]]
		if !ok {
			log.Printf("invalid verb: %q", fields[1])
			return
		}
		p = Pattern{
			Object: fields[0],
			Verb:   verb,
			Arg:    strings.Join(fields[2:], " "),
		}
		if p.Object == "" {
			log.Printf("invalid rule (missing object): %+v", p)
			continue
		} else if p.Arg == "" {
			log.Printf("invalid rule (missing argument): %+v", p.Arg)
			continue
		}
		rule.Patterns = append(rule.Patterns, p)
		capture = true
	}
	if s.Err() != nil {
		log.Println(s.Err())
		return
	}
	log.Println("no matching rule")
}
