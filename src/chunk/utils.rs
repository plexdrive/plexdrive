use std::cmp;

pub fn cut_chunk(chunk: &[u8], offset: u64, size: u64) -> Vec<u8> {
  trace!("Cutting Chunk to {} - {}", offset, (offset+size));

  let len = chunk.len() as u64;
  let offset_start = cmp::min(len, offset) as usize;
  let offset_end = cmp::min(len, offset + size) as usize;

  chunk[offset_start..offset_end].to_vec()
}