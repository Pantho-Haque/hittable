
.PHONY: install build test uninstall
install build test uninstall:
	$(MAKE) -C shellapp $@
