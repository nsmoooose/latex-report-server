#!/usr/bin/env python3
import os
import zipfile
import tempfile
import subprocess
from flask import Flask, request, send_file, jsonify
from werkzeug.utils import secure_filename

app = Flask(__name__)

@app.route("/compile", methods=["POST"])
def compile_latex():
    if "file" not in request.files:
        return jsonify({"error": "No file provided"}), 400

    file = request.files["file"]
    if file.filename == "" or not file.filename.endswith(".zip"):
        return jsonify({"error": "Invalid file type. Please upload a ZIP file."}), 400

    temp_dir = tempfile.mkdtemp()

    zip_path = os.path.join(temp_dir, secure_filename(file.filename))
    file.save(zip_path)

    # Extract the ZIP file
    try:
        with zipfile.ZipFile(zip_path, "r") as zip_ref:
            zip_ref.extractall(temp_dir)
    except zipfile.BadZipFile:
        return jsonify({"error": "Invalid ZIP file"}), 400

    tex_file = "document.tex"
    tex_path = os.path.join(temp_dir, tex_file)
    if not os.path.exists(tex_path):
        return jsonify({"error": "No document.tex found in the ZIP file"}), 400
    pdf_name = tex_file.rsplit(".", 1)[0] + ".pdf"
    pdf_path = os.path.join(temp_dir, pdf_name)

    # Run pdflatex
    try:
        subprocess.run(
            ["pdflatex", "-interaction=nonstopmode", "-output-directory", temp_dir, tex_path],
            check=True,
            stdout=subprocess.PIPE,
            stderr=subprocess.PIPE
        )
    except subprocess.CalledProcessError as e:
        return jsonify({"error": "LaTeX compilation failed", "details": e.stderr.decode()}), 500

    # Return the generated PDF
    return send_file(pdf_path, as_attachment=True)


if __name__ == "__main__":
    app.run(host="0.0.0.0", port=5000)
