Introduction
============

This is a small webserver that you can upload a zip file containing
LaTeX documents. The server will compile the content into a PDF file
returned to the client.

Development
===========

Install the following to be able to use pdflatex::

  # This is for Fedora
  sudo dnf install texlive-scheme-medium

Build the server::

  go build -ldflags="-s -w" -o latex-server main.go

Start the server with::

  ./latex-server

From one of the example documents you can now do::

  cd test_document2
  ./test.sh
  evince output.pdf

Testing the docker image
========================

Issue the following commands to build and start the server::

  # For podman based systems
  make podman
  make podman_run

  # For docker based systems
  make docker
  make docker_run

Then go back and test the example documents.
