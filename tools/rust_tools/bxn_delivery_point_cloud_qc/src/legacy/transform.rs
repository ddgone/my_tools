use crate::legacy::types::{MappingMode, OriginInfo, PivotAccumulator, PivotMode, TransformConfig};
use crate::legacy::types::{UTM_FALSE_EASTING, UTM_FALSE_NORTHING_SOUTH, UTM_K0, WGS84_A, WGS84_F};
use anyhow::{Context, Result, bail};
use las::{Builder, Header, Point, Vlr};
use std::f64::consts::PI;
use std::fs;
use std::path::Path;

pub(crate) fn read_origin(path: &Path) -> Result<OriginInfo> {
    let content = fs::read_to_string(path)
        .with_context(|| format!("无法读取 origin 文件: {}", path.display()))?;
    let parts: Vec<_> = content.split_whitespace().collect();
    if parts.len() < 3 {
        bail!("origin 文件至少需要包含 3 个值: lat lon alt");
    }
    Ok(OriginInfo {
        lat: parts[0].parse().context("origin 中的纬度解析失败")?,
        lon: parts[1].parse().context("origin 中的经度解析失败")?,
        alt: parts[2].parse().context("origin 中的高程解析失败")?,
    })
}

pub(crate) fn infer_utm_epsg(lat: f64, lon: f64) -> u16 {
    let zone = ((lon + 180.0) / 6.0).floor() as u16 + 1;
    if lat >= 0.0 {
        32600 + zone
    } else {
        32700 + zone
    }
}

pub(crate) fn apply_transform(point: &mut Point, config: &TransformConfig) -> Result<()> {
    let (x_rot, y_rot) = rotate_xy(
        point.x,
        point.y,
        config.pivot.0,
        config.pivot.1,
        config.yaw_deg,
    );
    match config.mapping {
        MappingMode::Flat => {
            point.x = config.origin_utm.0 + x_rot;
            point.y = config.origin_utm.1 + y_rot;
            point.z = config.origin_utm.2 + point.z;
        }
        MappingMode::Enu => {
            let (x_ecef, y_ecef, z_ecef) = enu_to_ecef(
                x_rot,
                y_rot,
                point.z,
                config.origin.lat,
                config.origin.lon,
                config.origin_ecef,
            );
            let (lat, lon, alt) = ecef_to_geodetic(x_ecef, y_ecef, z_ecef);
            let (x_utm, y_utm, z_utm) = utm_from_geodetic(lat, lon, alt, config.epsg)?;
            point.x = x_utm;
            point.y = y_utm;
            point.z = z_utm;
        }
    }
    Ok(())
}

fn rotate_xy(x: f64, y: f64, pivot_x: f64, pivot_y: f64, yaw_deg: f64) -> (f64, f64) {
    let theta = (-yaw_deg).to_radians();
    let cos_t = theta.cos();
    let sin_t = theta.sin();
    let dx = x - pivot_x;
    let dy = y - pivot_y;
    (
        dx * cos_t - dy * sin_t + pivot_x,
        dx * sin_t + dy * cos_t + pivot_y,
    )
}

pub(crate) fn ecef_from_geodetic(lat_deg: f64, lon_deg: f64, alt: f64) -> (f64, f64, f64) {
    let lat = lat_deg.to_radians();
    let lon = lon_deg.to_radians();
    let sin_lat = lat.sin();
    let cos_lat = lat.cos();
    let sin_lon = lon.sin();
    let cos_lon = lon.cos();
    let e2 = wgs84_e2();
    let n = WGS84_A / (1.0 - e2 * sin_lat * sin_lat).sqrt();
    let x = (n + alt) * cos_lat * cos_lon;
    let y = (n + alt) * cos_lat * sin_lon;
    let z = (n * (1.0 - e2) + alt) * sin_lat;
    (x, y, z)
}

fn enu_to_ecef(
    east: f64,
    north: f64,
    up: f64,
    lat_deg: f64,
    lon_deg: f64,
    origin_ecef: (f64, f64, f64),
) -> (f64, f64, f64) {
    let lat = lat_deg.to_radians();
    let lon = lon_deg.to_radians();
    let sin_lat = lat.sin();
    let cos_lat = lat.cos();
    let sin_lon = lon.sin();
    let cos_lon = lon.cos();
    let (x0, y0, z0) = origin_ecef;
    (
        x0 - sin_lon * east - sin_lat * cos_lon * north + cos_lat * cos_lon * up,
        y0 + cos_lon * east - sin_lat * sin_lon * north + cos_lat * sin_lon * up,
        z0 + cos_lat * north + sin_lat * up,
    )
}

fn ecef_to_geodetic(x: f64, y: f64, z: f64) -> (f64, f64, f64) {
    let e2 = wgs84_e2();
    let b = WGS84_A * (1.0 - WGS84_F);
    let ep2 = (WGS84_A * WGS84_A - b * b) / (b * b);
    let p = x.hypot(y);
    let lon = y.atan2(x);
    let theta = (z * WGS84_A).atan2(p * b);
    let sin_theta = theta.sin();
    let cos_theta = theta.cos();
    let lat = (z + ep2 * b * sin_theta.powi(3)).atan2(p - e2 * WGS84_A * cos_theta.powi(3));
    let sin_lat = lat.sin();
    let n = WGS84_A / (1.0 - e2 * sin_lat * sin_lat).sqrt();
    let alt = p / lat.cos() - n;
    (lat.to_degrees(), lon.to_degrees(), alt)
}

pub(crate) fn utm_from_geodetic(
    lat_deg: f64,
    lon_deg: f64,
    alt: f64,
    epsg: u16,
) -> Result<(f64, f64, f64)> {
    let (zone, northern) = epsg_to_utm_zone(epsg)?;
    let lat = lat_deg.to_radians();
    let lon = lon_deg.to_radians();
    let lon_origin = ((f64::from(zone) - 1.0) * 6.0 - 180.0 + 3.0).to_radians();
    let e2 = wgs84_e2();
    let ep2 = e2 / (1.0 - e2);
    let sin_lat = lat.sin();
    let cos_lat = lat.cos();
    let tan_lat = lat.tan();
    let n = WGS84_A / (1.0 - e2 * sin_lat * sin_lat).sqrt();
    let t = tan_lat * tan_lat;
    let c = ep2 * cos_lat * cos_lat;
    let a = cos_lat * (lon - lon_origin);
    let m = WGS84_A
        * ((1.0 - e2 / 4.0 - 3.0 * e2.powi(2) / 64.0 - 5.0 * e2.powi(3) / 256.0) * lat
            - (3.0 * e2 / 8.0 + 3.0 * e2.powi(2) / 32.0 + 45.0 * e2.powi(3) / 1024.0)
                * (2.0 * lat).sin()
            + (15.0 * e2.powi(2) / 256.0 + 45.0 * e2.powi(3) / 1024.0) * (4.0 * lat).sin()
            - (35.0 * e2.powi(3) / 3072.0) * (6.0 * lat).sin());
    let easting = UTM_FALSE_EASTING
        + UTM_K0
            * n
            * (a + (1.0 - t + c) * a.powi(3) / 6.0
                + (5.0 - 18.0 * t + t * t + 72.0 * c - 58.0 * ep2) * a.powi(5) / 120.0);
    let mut northing = UTM_K0
        * (m + n
            * tan_lat
            * (a.powi(2) / 2.0
                + (5.0 - t + 9.0 * c + 4.0 * c * c) * a.powi(4) / 24.0
                + (61.0 - 58.0 * t + t * t + 600.0 * c - 330.0 * ep2) * a.powi(6) / 720.0));
    if !northern {
        northing += UTM_FALSE_NORTHING_SOUTH;
    }
    Ok((easting, northing, alt))
}

pub(crate) fn epsg_to_utm_zone(epsg: u16) -> Result<(u8, bool)> {
    match epsg {
        32601..=32660 => Ok(((epsg - 32600) as u8, true)),
        32701..=32760 => Ok(((epsg - 32700) as u8, false)),
        _ => bail!("目前仅支持 WGS84 UTM EPSG:32601-32660 或 EPSG:32701-32760"),
    }
}

pub(crate) fn build_transformed_header(
    original_header: &Header,
    points: &[Point],
    epsg: u16,
) -> Result<Header> {
    let mut builder = Builder::from(original_header.clone());
    builder.version = original_header.version();
    builder.vlrs.retain(|vlr| !vlr.is_crs());
    if let Some((min_x, min_y, min_z)) = min_coordinates(points) {
        builder.transforms.x.offset = min_x.floor();
        builder.transforms.y.offset = min_y.floor();
        builder.transforms.z.offset = min_z.floor();
    }
    builder.vlrs.extend(build_geotiff_crs_vlrs(epsg)?);
    let mut header = builder
        .into_header()
        .context("重建带 CRS 的 LAS header 失败")?;
    header.clear();
    for point in points {
        header.add_point(point);
    }
    Ok(header)
}

fn build_geotiff_crs_vlrs(epsg: u16) -> Result<Vec<Vlr>> {
    let citation = build_utm_citation(epsg)?;
    let ascii_bytes = citation.as_bytes();
    let ascii_len: u16 = ascii_bytes
        .len()
        .try_into()
        .context("GeoTIFF citation 过长")?;
    let mut geokey_data = Vec::with_capacity(32);
    for value in [
        1_u16, 1, 0, 3, 1024, 0, 1, 1, 3072, 0, 1, epsg, 3073, 34737, ascii_len, 0,
    ] {
        geokey_data.extend_from_slice(&value.to_le_bytes());
    }
    Ok(vec![
        Vlr {
            user_id: "LASF_Projection".to_string(),
            record_id: 34735,
            description: "GeoTIFF GeoKeyDirectoryTag".to_string(),
            data: geokey_data,
        },
        Vlr {
            user_id: "LASF_Projection".to_string(),
            record_id: 34737,
            description: "GeoTIFF GeoAsciiParamsTag".to_string(),
            data: ascii_bytes.to_vec(),
        },
    ])
}

pub(crate) fn rebuild_header_for_points(template: &Header, points: &[Point]) -> Result<Header> {
    let mut builder = Builder::from(template.clone());
    if let Some((min_x, min_y, min_z)) = min_coordinates(points) {
        builder.transforms.x.offset = min_x.floor();
        builder.transforms.y.offset = min_y.floor();
        builder.transforms.z.offset = min_z.floor();
    }
    let mut header = builder.into_header().context("重建 LAS header 失败")?;
    header.clear();
    for point in points {
        header.add_point(point);
    }
    Ok(header)
}

fn min_coordinates(points: &[Point]) -> Option<(f64, f64, f64)> {
    if points.is_empty() {
        return None;
    }
    let mut min_x = f64::INFINITY;
    let mut min_y = f64::INFINITY;
    let mut min_z = f64::INFINITY;
    for point in points {
        min_x = min_x.min(point.x);
        min_y = min_y.min(point.y);
        min_z = min_z.min(point.z);
    }
    Some((min_x, min_y, min_z))
}

pub(crate) fn build_utm_wkt(epsg: u16) -> Result<String> {
    let (zone, northern) = epsg_to_utm_zone(epsg)?;
    let central_meridian = (i32::from(zone) - 1) * 6 - 180 + 3;
    let false_northing = if northern {
        0.0
    } else {
        UTM_FALSE_NORTHING_SOUTH
    };
    let hemisphere = if northern { "N" } else { "S" };
    Ok(format!(
        "PROJCS[\"WGS 84 / UTM zone {zone}{hemisphere}\",GEOGCS[\"WGS 84\",DATUM[\"WGS_1984\",SPHEROID[\"WGS 84\",6378137,298.257223563]],PRIMEM[\"Greenwich\",0],UNIT[\"degree\",0.0174532925199433]],PROJECTION[\"Transverse_Mercator\"],PARAMETER[\"latitude_of_origin\",0],PARAMETER[\"central_meridian\",{central_meridian}],PARAMETER[\"scale_factor\",0.9996],PARAMETER[\"false_easting\",500000],PARAMETER[\"false_northing\",{false_northing}],UNIT[\"metre\",1]]"
    ))
}

fn build_utm_citation(epsg: u16) -> Result<String> {
    let (zone, northern) = epsg_to_utm_zone(epsg)?;
    Ok(format!(
        "WGS 84 / UTM zone {}{}",
        zone,
        if northern { "N" } else { "S" }
    ))
}

pub(crate) fn update_pivot_accumulator(acc: &mut PivotAccumulator, point: &Point) {
    acc.count += 1;
    acc.sum_x += point.x;
    acc.sum_y += point.y;
    acc.min_x = acc.min_x.min(point.x);
    acc.min_y = acc.min_y.min(point.y);
    acc.max_x = acc.max_x.max(point.x);
    acc.max_y = acc.max_y.max(point.y);
}

pub(crate) fn pivot_from_accumulator(acc: &PivotAccumulator, mode: PivotMode) -> (f64, f64) {
    if acc.count == 0 || mode == PivotMode::Zero {
        return (0.0, 0.0);
    }
    match mode {
        PivotMode::Centroid => (acc.sum_x / acc.count as f64, acc.sum_y / acc.count as f64),
        PivotMode::BboxCenter => ((acc.min_x + acc.max_x) * 0.5, (acc.min_y + acc.max_y) * 0.5),
        PivotMode::Zero => (0.0, 0.0),
    }
}

pub(crate) fn merge_pivot_accumulator(target: &mut PivotAccumulator, other: &PivotAccumulator) {
    if other.count == 0 {
        return;
    }
    if target.count == 0 {
        *target = other.clone();
        return;
    }
    target.count += other.count;
    target.sum_x += other.sum_x;
    target.sum_y += other.sum_y;
    target.min_x = target.min_x.min(other.min_x);
    target.min_y = target.min_y.min(other.min_y);
    target.max_x = target.max_x.max(other.max_x);
    target.max_y = target.max_y.max(other.max_y);
}

pub(crate) fn voxel_shard(key: crate::legacy::types::VoxelKey, shard_count: usize) -> usize {
    if shard_count <= 1 {
        return 0;
    }
    let mut hash = key.x as i64 as u64;
    hash = hash.wrapping_mul(0x9E37_79B1_85EB_CA87);
    hash ^= (key.y as i64 as u64).rotate_left(21);
    hash = hash.wrapping_mul(0xC2B2_AE3D_27D4_EB4F);
    hash ^= (key.z as i64 as u64).rotate_left(42);
    (hash as usize) % shard_count
}

pub(crate) fn point_order(frame_index: usize, point_index: usize) -> u64 {
    ((frame_index as u64) << 32) | point_index as u64
}

pub(crate) fn build_transform_config(
    origin_path: &Path,
    epsg_override: Option<u16>,
    yaw_deg: f64,
    mapping: MappingMode,
    pivot: (f64, f64),
) -> Result<TransformConfig> {
    let origin = read_origin(origin_path)?;
    let epsg = epsg_override.unwrap_or_else(|| infer_utm_epsg(origin.lat, origin.lon));
    let origin_utm = utm_from_geodetic(origin.lat, origin.lon, origin.alt, epsg)?;
    let origin_ecef = ecef_from_geodetic(origin.lat, origin.lon, origin.alt);
    Ok(TransformConfig {
        origin,
        epsg,
        origin_utm,
        origin_ecef,
        pivot,
        yaw_deg,
        mapping,
    })
}

pub(crate) fn wgs84_e2() -> f64 {
    WGS84_F * (2.0 - WGS84_F)
}

#[allow(dead_code)]
pub(crate) fn rad_to_deg(rad: f64) -> f64 {
    rad * 180.0 / PI
}
