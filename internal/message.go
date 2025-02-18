package internal

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"strconv"
	"strings"
)

type Message struct {
	Src  string // application/service generating message
	Dst  string // destination 'port' for message
	Wdir string // working directory (used if data is a file name)
	Type string // form of the data, e.g. text (only supported for now)
	// Attributes of the message, in name=value
	// pairs separated by white space (the value must follow the
	// usual quoting convention if it contains white space or quote
	// characters or equal signs; it cannot contain a newline)
	Attr map[string]string
	Data []byte // the data itself
}

func (m *Message) String() string {
	return fmt.Sprintf("src=%q dst=%q wdir=%q type=%q attr=%q data=%q",
		m.Src, m.Dst, m.Wdir, m.Type, m.Attr, m.Data)
}

func (m *Message) Encode(w io.Writer) error {
	var b bytes.Buffer
	b.WriteString(m.Src + "\n")
	b.WriteString(m.Dst + "\n")
	b.WriteString(m.Wdir + "\n")
	b.WriteString(m.Type + "\n")
	i := 0
	for a, v := range m.Attr {
		fmt.Fprintf(&b, "%s='%s'", a, v)
		i++
		if i < len(m.Attr) {
			fmt.Fprint(&b, " ")
		} else {
			fmt.Fprint(&b, "\n")
		}
	}
	if len(m.Attr) == 0 {
		fmt.Fprint(&b, "\n")
	}
	fmt.Fprintf(&b, "%d\n", len(m.Data))
	b.Write(m.Data)
	_, err := w.Write(b.Bytes())
	return err
	//return gob.NewEncoder(w).Encode(m)
}

func readLine(r *bufio.Reader) (string, error) {
	ln, err := r.ReadString('\n')
	if err != nil {
		return "", err
	}
	return strings.Trim(ln, "\n"), nil
}

func (m *Message) Decode(r io.Reader) error {
	var (
		err error
		br  = bufio.NewReader(r)
	)
	if m.Src, err = readLine(br); err != nil {
		return err
	}
	if m.Dst, err = readLine(br); err != nil {
		return err
	}
	if m.Wdir, err = readLine(br); err != nil {
		return err
	}
	if m.Type, err = readLine(br); err != nil {
		return err
	}
	m.Attr = make(map[string]string)
	for {
		ln, err := readLine(br)
		if err != nil {
			return err
		}
		if ln == "" {
			break
		}
		fields := strings.Split(ln, "=")
		if len(fields) < 1 {
			return fmt.Errorf("invalid attribute syntax: %q", ln)
		}
		m.Attr[fields[0]] = strings.Join(fields[1:], "=")
	}
	ndata, err := readLine(br)
	if err != nil {
		return err
	}
	n, err := strconv.Atoi(ndata)
	if err != nil {
		return err
	}
	m.Data = make([]byte, n)
	_, err = br.Read(m.Data)
	return err
	//return gob.NewDecoder(r).Decode(m)
}
