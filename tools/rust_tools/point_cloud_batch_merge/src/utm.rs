use anyhow::{Result, bail};
use las::Vlr;

const WGS84_A: f64 = 6_378_137.0;
const WGS84_F: f64 = 1.0 / 298.257_223_563;
const UTM_K0: f64 = 0.9996;
const UTM_FALSE_EASTING: f64 = 500_000.0;
const UTM_FALSE_NORTHING_SOUTH: f64 = 10_000_000.0;

/// WGS84 经纬度 → UTM (easting, northing, elevation)
pub fn wgs84_to_utm(lat_deg: f64, lon_deg: f64, alt: f64, epsg: u16) -> Result<(f64, f64, f64)> {
    let (zone, northern) = epsg_to_zone(epsg)?;
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
            * (a
                + (1.0 - t + c) * a.powi(3) / 6.0
                + (5.0 - 18.0 * t + t * t + 72.0 * c - 58.0 * ep2) * a.powi(5) / 120.0);

    let mut northing = UTM_K0
        * (m
            + n * tan_lat
                * (a.powi(2) / 2.0
                    + (5.0 - t + 9.0 * c + 4.0 * c * c) * a.powi(4) / 24.0
                    + (61.0 - 58.0 * t + t * t + 600.0 * c - 330.0 * ep2) * a.powi(6)
                        / 720.0));

    if !northern {
        northing += UTM_FALSE_NORTHING_SOUTH;
    }

    Ok((easting, northing, alt))
}

/// 从 lat/lon 推断 UTM EPSG 码
pub fn infer_epsg(lat_deg: f64, lon_deg: f64) -> u16 {
    let zone = ((lon_deg + 180.0) / 6.0).floor() as u16 + 1;
    if lat_deg >= 0.0 {
        32600 + zone
    } else {
        32700 + zone
    }
}

fn epsg_to_zone(epsg: u16) -> Result<(u8, bool)> {
    match epsg {
        32601..=32660 => Ok(((epsg - 32600) as u8, true)),
        32701..=32760 => Ok(((epsg - 32700) as u8, false)),
        _ => bail!("仅支持 WGS84 UTM EPSG:32601-32660 或 32701-32760"),
    }
}

fn wgs84_e2() -> f64 {
    WGS84_F * (2.0 - WGS84_F)
}

/// WGS84 经纬度 → ECEF (地心地固坐标)
fn geodetic_to_ecef(lat_deg: f64, lon_deg: f64, alt: f64) -> (f64, f64, f64) {
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

/// ENU 米偏移 → ECEF (以 origin 为参考点)
fn enu_to_ecef(east: f64, north: f64, up: f64, origin_lat: f64, origin_lon: f64, origin_ecef: (f64, f64, f64)) -> (f64, f64, f64) {
    let lat = origin_lat.to_radians();
    let lon = origin_lon.to_radians();
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

/// ECEF → WGS84 经纬度
fn ecef_to_geodetic(x: f64, y: f64, z: f64) -> (f64, f64, f64) {
    let e2 = wgs84_e2();
    let b = WGS84_A * (1.0 - WGS84_F);
    let ep2 = (WGS84_A * WGS84_A - b * b) / (b * b);
    let p = x.hypot(y);
    let lon = y.atan2(x).to_degrees();
    let theta = (z * WGS84_A).atan2(p * b);
    let sin_theta = theta.sin();
    let cos_theta = theta.cos();
    let lat = (z + ep2 * b * sin_theta.powi(3)).atan2(p - e2 * WGS84_A * cos_theta.powi(3)).to_degrees();
    let sin_lat = lat.to_radians().sin();
    let n = WGS84_A / (1.0 - e2 * sin_lat * sin_lat).sqrt();
    let alt = p / lat.to_radians().cos() - n;
    (lat, lon, alt)
}

/// ENU 米偏移 → WGS84(lon, lat, alt)（精密测地线转换）
pub fn enu_to_wgs84(enu_x: f64, enu_y: f64, enu_z: f64, origin_lat: f64, origin_lon: f64, origin_alt: f64) -> (f64, f64, f64) {
    let origin_ecef = geodetic_to_ecef(origin_lat, origin_lon, origin_alt);
    let (x, y, z) = enu_to_ecef(enu_x, enu_y, enu_z, origin_lat, origin_lon, origin_ecef);
    let (lat, lon, alt) = ecef_to_geodetic(x, y, z);
    (lon, lat, alt)
}

/// 构建 UTM 投影的 GeoTIFF VLR 记录
pub fn build_utm_vlrs(epsg: u16) -> Result<Vec<Vlr>> {
    let (zone, northern) = epsg_to_zone(epsg)?;
    let hemisphere = if northern { "N" } else { "S" };
    let central_meridian = (i32::from(zone) - 1) * 6 - 180 + 3;
    let false_northing = if northern { 0.0 } else { UTM_FALSE_NORTHING_SOUTH };

    let citation = format!("WGS 84 / UTM zone {zone}{hemisphere}");
    let ascii_bytes = citation.as_bytes();
    let ascii_len: u16 = ascii_bytes.len().try_into().unwrap();

    let wkt = format!(
        "PROJCS[\"WGS 84 / UTM zone {zone}{hemisphere}\",GEOGCS[\"WGS 84\",DATUM[\"WGS_1984\",SPHEROID[\"WGS 84\",6378137,298.257223563]],PRIMEM[\"Greenwich\",0],UNIT[\"degree\",0.0174532925199433]],PROJECTION[\"Transverse_Mercator\"],PARAMETER[\"latitude_of_origin\",0],PARAMETER[\"central_meridian\",{central_meridian}],PARAMETER[\"scale_factor\",0.9996],PARAMETER[\"false_easting\",500000],PARAMETER[\"false_northing\",{false_northing}],UNIT[\"metre\",1]]"
    );
    let wkt_bytes = wkt.as_bytes();

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
        Vlr {
            user_id: "LASF_Projection".to_string(),
            record_id: 2112,
            description: "OGC WKT Coordinate System".to_string(),
            data: wkt_bytes.to_vec(),
        },
    ])
}
