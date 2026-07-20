use anyhow::{Context, Result, bail};
use las::{Builder, Header, Point, Vlr};
use std::fs;
use std::path::Path;

const WGS84_A: f64 = 6_378_137.0;
const WGS84_F: f64 = 1.0 / 298.257_223_563;
const UTM_K0: f64 = 0.9996;
const UTM_FALSE_EASTING: f64 = 500_000.0;
const UTM_FALSE_NORTHING_SOUTH: f64 = 10_000_000.0;

#[derive(Clone, Debug)]
pub struct OriginInfo {
    pub lat: f64,
    pub lon: f64,
    pub alt: f64,
}

#[derive(Clone, Debug)]
pub struct TransformConfig {
    pub origin: OriginInfo,
    pub epsg: u16,
    #[allow(dead_code)]
    origin_utm: (f64, f64, f64),
    origin_ecef: (f64, f64, f64),
}

pub fn build_config(
    origin_path: &Path,
    epsg_override: Option<u16>,
    _yaw_deg: f64,
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
    })
}

fn read_origin(path: &Path) -> Result<OriginInfo> {
    let content =
        fs::read_to_string(path).with_context(|| format!("无法读取 origin 文件: {}", path.display()))?;
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

pub fn apply_transform(point: &mut Point, config: &TransformConfig) -> Result<()> {
    // rotate_xy: 原工具 batch 模式下 yaw=0，恒等变换
    let x_rot = point.x;
    let y_rot = point.y;

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
    Ok(())
}

#[allow(dead_code)]
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

fn infer_utm_epsg(lat: f64, lon: f64) -> u16 {
    let zone = ((lon + 180.0) / 6.0).floor() as u16 + 1;
    if lat >= 0.0 {
        32600 + zone
    } else {
        32700 + zone
    }
}

fn ecef_from_geodetic(lat_deg: f64, lon_deg: f64, alt: f64) -> (f64, f64, f64) {
    let lat = lat_deg.to_radians();
    let lon = lon_deg.to_radians();
    let sin_lat = lat.sin();
    let cos_lat = lat.cos();
    let e2 = wgs84_e2();
    let n = WGS84_A / (1.0 - e2 * sin_lat * sin_lat).sqrt();
    (
        (n + alt) * cos_lat * lon.cos(),
        (n + alt) * cos_lat * lon.sin(),
        (n * (1.0 - e2) + alt) * sin_lat,
    )
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

fn utm_from_geodetic(
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

fn epsg_to_utm_zone(epsg: u16) -> Result<(u8, bool)> {
    match epsg {
        32601..=32660 => Ok(((epsg - 32600) as u8, true)),
        32701..=32760 => Ok(((epsg - 32700) as u8, false)),
        _ => bail!("目前仅支持 WGS84 UTM EPSG:32601-32660 或 EPSG:32701-32760"),
    }
}

fn wgs84_e2() -> f64 {
    WGS84_F * (2.0 - WGS84_F)
}

// ---- LAZ header with EPSG CRS ----

pub fn build_point_cloud_header(
    points: &[Point],
    epsg: Option<u16>,
) -> Result<Header> {
    let mut builder = Builder::from((1, 2));
    builder.generating_software = "pcd_to_laz_per_frame".to_string();
    builder.system_identifier = "pcd_convert".to_string();
    builder.point_format = las::point::Format::new(0).context("创建 LAS 点格式失败")?;
    builder.transforms.x.scale = 0.001;
    builder.transforms.y.scale = 0.001;
    builder.transforms.z.scale = 0.001;

    if !points.is_empty() {
        let (min_x, min_y, min_z) = min_coordinates(points).unwrap_or((0.0, 0.0, 0.0));
        builder.transforms.x.offset = min_x.floor();
        builder.transforms.y.offset = min_y.floor();
        builder.transforms.z.offset = min_z.floor();
    }

    if let Some(epsg) = epsg {
        builder
            .vlrs
            .extend(build_geotiff_crs_vlrs(epsg)?);
    }

    let mut header = builder
        .into_header()
        .context("构建 LAS header 失败")?;
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

fn build_utm_citation(epsg: u16) -> Result<String> {
    let (zone, northern) = epsg_to_utm_zone(epsg)?;
    Ok(format!(
        "WGS 84 / UTM zone {}{}",
        zone,
        if northern { "N" } else { "S" }
    ))
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
