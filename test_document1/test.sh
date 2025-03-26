#!/bin/bash
[[ -f report.zip ]] && rm report.zip
zip report.zip *
curl -X POST -F "file=@report.zip" http://localhost:5000/compile --output output.pdf
