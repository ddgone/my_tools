use anyhow::Result;

pub mod batch;
pub mod legacy;

pub fn run(args: &[String]) -> Result<()> {
    batch::run_batch_cli_from(args)
}
