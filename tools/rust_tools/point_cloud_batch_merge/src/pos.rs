use serde::Deserialize;
use std::collections::HashMap;
use std::fs;
use std::path::Path;

#[derive(Debug, Deserialize)]
struct PosJson {
    #[serde(rename = "pointList")]
    point_list: Vec<PosPoint>,
}

#[derive(Debug, Deserialize)]
struct PosPoint {
    timestamp: u64,
    x: f64,
    y: f64,
    z: f64,
    #[allow(dead_code)]
    azimuth: f64,
    #[allow(dead_code)]
    pitch: f64,
    #[allow(dead_code)]
    roll: f64,
}

#[derive(Debug, Clone)]
pub struct PosPose {
    pub x: f64,
    pub y: f64,
    pub z: f64,
    pub azimuth_deg: f64,
}

pub fn load_pos(path: &Path) -> anyhow::Result<HashMap<u64, PosPose>> {
    let content = fs::read_to_string(path)?;
    let parsed: PosJson = serde_json::from_str(&content)?;
    let mut map = HashMap::with_capacity(parsed.point_list.len());
    for point in parsed.point_list {
        map.insert(
            point.timestamp,
            PosPose {
                x: point.x,
                y: point.y,
                z: point.z,
                azimuth_deg: point.azimuth,
            },
        );
    }
    Ok(map)
}
