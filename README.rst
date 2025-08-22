Introduction
============

This is a small webserver that you can upload a zip file containing
LaTeX documents. The server will compile the content into a PDF file
returned to the client.

Development
===========

This will create a virtual environment to run the server with::

  virtualenv venv
  source venv/bin/activate
  pip install -r requirements.txt

Install the following to be able to use pdflatex::

  # This is for Fedora
  sudo dnf install texlive-scheme-medium

Start the server with::

  ./server.py

From one of the example documents you can now do::

  cd test_document2
  ./test.sh
  evince output.pdf
