import argparse
import json
import os
import platform
import site
import sys
from pathlib import Path

import numpy as np
import requests
from PIL import Image

img = Image.new('RGB', (100, 100), color='red')

from bs4 import BeautifulSoup
soup = BeautifulSoup("<p>test</p>", "html.parser")


def parse_args():
    parser = argparse.ArgumentParser(description="Python environment diagnostics")
    parser.add_argument("-output", required=True, help="Path to diagnostic JSON output")
    parser.add_argument("-count", type=int, default=1024, help="Sample count for numeric verification")
    parser.add_argument("-label", default="", help="Optional report label")
    return parser.parse_args()


def build_report(args):
    count = max(8, int(args.count))
    samples = np.linspace(0.0, 1.0, num=count, dtype=np.float64)
    values = np.sin(samples * np.pi) + np.cos(samples * np.pi * 0.5)
    checksum = float(np.round(values.sum(), 8))
    matrix = np.array([
        [1.0, 2.0, 3.0],
        [0.5, 1.5, 2.5],
        [2.0, 1.0, 0.25],
    ], dtype=np.float64)
    determinant = float(np.round(np.linalg.det(matrix), 8))
    headers = requests.utils.default_headers()

    return {
        "label": args.label,
        "python": {
            "executable": sys.executable,
            "version": sys.version,
            "version_info": list(sys.version_info[:3]),
            "prefix": sys.prefix,
            "base_prefix": getattr(sys, "base_prefix", ""),
            "exec_prefix": sys.exec_prefix,
            "platform": platform.platform(),
            "cwd": os.getcwd(),
            "argv": sys.argv,
            "path_head": sys.path[:8],
            "site_packages": site.getsitepackages() if hasattr(site, "getsitepackages") else [],
            "user_site": site.getusersitepackages() if hasattr(site, "getusersitepackages") else "",
            "virtual_env": os.environ.get("VIRTUAL_ENV", ""),
        },
        "packages": {
            "numpy": np.__version__,
            "requests": requests.__version__,
        },
        "requests": {
            "default_user_agent": headers.get("User-Agent", ""),
            "accept_encoding": headers.get("Accept-Encoding", ""),
        },
        "calculation": {
            "sample_count": count,
            "checksum": checksum,
            "mean": float(np.round(values.mean(), 8)),
            "stddev": float(np.round(values.std(), 8)),
            "determinant": determinant,
            "first_values": [float(np.round(item, 8)) for item in values[:6]],
        },
    }


def main():
    args = parse_args()
    output_path = Path(args.output).expanduser().resolve()
    output_path.parent.mkdir(parents=True, exist_ok=True)
    report = build_report(args)

    print("=== Python 环境诊断 ===")
    print(f"sys.executable = {report['python']['executable']}")
    print(f"sys.prefix = {report['python']['prefix']}")
    print(f"base_prefix = {report['python']['base_prefix']}")
    print(f"VIRTUAL_ENV = {report['python']['virtual_env']}")
    print(f"numpy = {report['packages']['numpy']}")
    print(f"requests = {report['packages']['requests']}")
    print(f"checksum = {report['calculation']['checksum']}")
    print(f"determinant = {report['calculation']['determinant']}")
    print(f"output = {output_path}")

    output_path.write_text(json.dumps(report, indent=2, ensure_ascii=False), encoding="utf-8")
    print("诊断文件已写出")


if __name__ == "__main__":
    main()
