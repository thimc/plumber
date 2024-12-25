# Plumber

Yet another plumber tool for Unix.

## Install

	$ make
	# make install

	$ plumber &

	...

	$ plumb 'https://google.com'

## Updating the rule set

Edit the rules file directly (defaults to `/mnt/plumb/rules`) or modify
the file in the repository and run `make rules`. The rules file is read
every time a new plumb message is received, so there is no need for
restarting the plumber after modifying the file.

## Todo

* The `attr` field is unused.

Write man pages for:

 * plumb(1) - tool to interface with the plumber(1) daemon
 * plumber(1) - daemon for interprocess messaging
 * rules(5) - plumber rule file format

