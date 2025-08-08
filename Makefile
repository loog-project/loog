build:
	@echo "Building loog..."
	go install .

unlink-kubectl:
	@echo "Unlinking kubectl-observe plugin..."
	sudo rm -f /usr/local/bin/kubectl-observe
	sudo rm -f /usr/local/bin/kubectl_complete-observe
	@echo "Unlinking complete!"

link-kubectl: build
	@echo "Installing kubectl-observe plugin..."
	sudo ln -s $(shell which loog) /usr/local/bin/kubectl-observe
	sudo chmod +x /usr/local/bin/kubectl-observe

	@echo "Installing kubectl-observe completion script..."
	sudo ln -s $(shell pwd)/compat/kubectl/kubectl_complete-observe /usr/local/bin/kubectl_complete-observe
	sudo chmod +x /usr/local/bin/kubectl_complete-observe

	@echo "Installation complete! You can now use 'kubectl observe' command."
