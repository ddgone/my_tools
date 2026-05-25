import open3d as o3d
import laspy
import numpy as np
import os
import re
import argparse
import glob
import mgrs
from pyproj import CRS, Transformer
import tempfile
import shutil


def get_mgrs_utm_info(filename):
    match = re.search(r'(\d{1,2}[C-X][A-Z]{2}\d+)', filename)
    if not match:
        return None

    mgrs_str = match.group(1)
    m = mgrs.MGRS()

    res_match = re.search(r'(\d{1,2}[C-X][A-Z]{2})(\d+)', mgrs_str)
    if res_match:
        grid_zone = res_match.group(1)
        digits = res_match.group(2)
        half_len = len(digits) // 2
        km_x = digits[:2] if len(digits) >= 2 else "00"
        km_y = digits[half_len:half_len+2] if len(digits) >= 2 else "00"
        mgrs_1km_bl = f"{grid_zone}{km_x}000{km_y}000"
    else:
        mgrs_1km_bl = mgrs_str

    try:
        lat, lon = m.toLatLon(mgrs_1km_bl)

        zone_num = int(re.match(r'\d+', grid_zone).group())
        band = grid_zone[len(str(zone_num))]
        is_north = band >= 'N'
        epsg = (32600 if is_north else 32700) + zone_num

        transformer = Transformer.from_crs("EPSG:4326", f"EPSG:{epsg}", always_xy=True)
        offset_x, offset_y = transformer.transform(lon, lat)

        offset_x = round(offset_x)
        offset_y = round(offset_y)

        return {
            "epsg": epsg,
            "offset_x": offset_x,
            "offset_y": offset_y,
            "mgrs_bl": mgrs_1km_bl
        }
    except Exception as e:
        print(f"Error parsing MGRS {mgrs_str}: {e}")
        return None


def process_pcd_to_las(input_pcd, output_las):
    input_pcd = os.path.normpath(input_pcd)
    filename = os.path.basename(input_pcd)
    info = get_mgrs_utm_info(filename)

    if not info:
        print(f"Skipping {filename}: Not a valid MGRS-style filename.")
        return False

    print(f"Processing: {filename}")
    print(f"  -> MGRS BL: {info['mgrs_bl']}")
    print(f"  -> Dynamic Offset: X={info['offset_x']}, Y={info['offset_y']} (EPSG:{info['epsg']})")

    temp_dir = None
    try:
        has_non_ascii = any(ord(c) > 127 for c in input_pcd)

        if has_non_ascii:
            temp_dir = tempfile.mkdtemp()
            temp_filename = os.path.join(temp_dir, filename)
            shutil.copy2(input_pcd, temp_filename)
            pcd_path_to_use = temp_filename
        else:
            pcd_path_to_use = input_pcd

        pcd = o3d.io.read_point_cloud(pcd_path_to_use)

    except Exception as e:
        print(f"  -> Error reading PCD file: {e}")
        return False
    finally:
        if temp_dir and os.path.exists(temp_dir):
            try:
                shutil.rmtree(temp_dir)
            except:
                pass

    points = np.asarray(pcd.points)

    if len(points) == 0:
        print(f"  -> Error: No points found.")
        return False

    restored_x = points[:, 0] + info['offset_x']
    restored_y = points[:, 1] + info['offset_y']
    restored_z = points[:, 2]

    header = laspy.LasHeader(point_format=3, version="1.4")
    header.scales = [0.01, 0.01, 0.01]
    header.offsets = [info['offset_x'], info['offset_y'], 0]

    try:
        crs_utm = CRS.from_epsg(info['epsg'])
        header.add_crs(crs_utm)
    except Exception as e:
        print(f"  -> Warning: Could not set CRS: {e}")

    las = laspy.LasData(header)
    las.x = restored_x
    las.y = restored_y
    las.z = restored_z

    if pcd.has_colors():
        colors = np.asarray(pcd.colors) * 65535
        las.red = colors[:, 0].astype(np.uint16)
        las.green = colors[:, 1].astype(np.uint16)
        las.blue = colors[:, 2].astype(np.uint16)

    las.update_header()
    las.write(output_las)
    print(f"  -> Successfully saved to {output_las}\n")
    return True


def main():
    parser = argparse.ArgumentParser(
        description="白犀牛偏转后的PCD文件转换回未偏转的LAS文件"
    )
    parser.add_argument(
        "-input",
        required=True,
        help="包含PCD文件的目录（PCD命名必须为百米块MGRS格式，如 50QKL416457.pcd）"
    )
    parser.add_argument(
        "-output",
        default=None,
        help="输出LAS文件的目录，默认在输入目录下创建 output 子文件夹"
    )
    parser.add_argument(
        "-workers",
        type=int,
        default=1,
        help="并行线程数（默认1）"
    )

    args = parser.parse_args()

    input_path = os.path.normpath(args.input)

    if os.path.isdir(input_path):
        files = glob.glob(os.path.join(input_path, "*.pcd"))
    elif os.path.isfile(input_path):
        files = [input_path]
    else:
        print(f"Error: Invalid input path: {input_path}")
        return

    if not files:
        print(f"Error: No .pcd files found in {input_path}")
        return

    if args.output is not None:
        output_dir = os.path.normpath(args.output)
    elif os.path.isdir(input_path):
        output_dir = os.path.join(input_path, "output")
    else:
        output_dir = os.path.join(os.path.dirname(input_path), "output")

    os.makedirs(output_dir, exist_ok=True)

    print(f"Input directory : {input_path}")
    print(f"Output directory: {output_dir}")
    print(f"Total .pcd files: {len(files)}")
    if args.workers > 1:
        print(f"Workers: {args.workers}")
    print()

    from concurrent.futures import ThreadPoolExecutor, as_completed

    success_count = 0
    fail_count = 0

    def process_one(f):
        filename = os.path.basename(f)
        output_las = os.path.join(output_dir, filename.replace('.pcd', '_restored.las'))
        return process_pcd_to_las(f, output_las)

    with ThreadPoolExecutor(max_workers=args.workers) as executor:
        futures = {executor.submit(process_one, f): f for f in files}
        for future in as_completed(futures):
            if future.result():
                success_count += 1
            else:
                fail_count += 1

    print(f"Done! Success: {success_count}, Failed: {fail_count}")


if __name__ == "__main__":
    main()
