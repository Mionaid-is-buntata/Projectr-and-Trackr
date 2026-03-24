# Installed on the Pi as /opt/projctr/Makefile (copied by `make deploy` from the workstation).
# The full deploy pipeline (cross-compile + rsync + restart) runs only from the dev repo.

.DEFAULT_GOAL := help

.PHONY: help deploy restart status logs

help:
	@echo "Projctr install directory (/opt/projctr)."
	@echo ""
	@echo "  Update: run 'make deploy' from your workstation (full git repo), not here."
	@echo ""
	@echo "  Local commands:"
	@echo "    make restart   — restart projctr systemd unit"
	@echo "    make status    — systemd status"
	@echo "    make logs      — last 100 journal lines"
	@echo ""

deploy:
	@echo "make deploy must be run from the workstation with the full Projctr repository." >&2
	@echo "It cross-compiles for ARM64 and rsyncs this directory. See README.md — Deployment." >&2
	@exit 1

restart:
	sudo systemctl restart projctr

status:
	sudo systemctl status projctr --no-pager

logs:
	sudo journalctl -u projctr -n 100 --no-pager
