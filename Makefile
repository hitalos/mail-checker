SHELL=/bin/bash

help:
	@echo "Usage:"
	@sed -n 's/^## //p' ${MAKEFILE_LIST} | column -t -s ':' | sed -e 's/^/  /'

## build: compile the program
build:
	CGO_ENABLED=0 go build -ldflags '-s -w' -trimpath -o dist/mail-checker .

## run: execute this project
run:
	@go run .

## install: install the binary to ~/bin/go
install: build
	go install .

install-systemd-user:
	cp systemd/mail-checker.* ~/.config/systemd/user/
	systemctl --user enable mail-checker.service
	systemctl --user enable --now mail-checker.timer
	systemctl --user daemon-reload

uninstall-systemd-user:
	rm ~/.config/systemd/user/mail-checker.{service,timer}
	systemctl --user daemon-reload

## clean: clean up the built files
clean: confirm
	@echo "Cleaning up…"
	@rm -rf dist/

confirm:
	@echo -n 'Are you sure? [y/N] ' && read ans && [[ $${ans:-N} == 'y' ]]

.PHONY: build clean confirm help run