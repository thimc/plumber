DIR=/mnt/plumb

all: plumb plumber

plumb: cmd/plumb/main.go
	go build -o plumb cmd/plumb/main.go

plumber: cmd/plumber/main.go
	go build -o plumber cmd/plumber/main.go

install: rules plumb plumber
	cp plumb plumber $(HOME)/bin/

rules:
	@if test ! -d $(DIR); then \
		mkdir -p $(DIR);   \
	fi
	cp rules $(DIR)/rules

clean:
	@rm plumb plumber

.PHONY: all clean rules
