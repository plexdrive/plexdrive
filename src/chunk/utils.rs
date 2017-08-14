use std::cmp;

pub fn cut_chunk(chunk: &[u8], offset: usize, size: usize) -> Vec<u8> {
  let len = chunk.len();

  let offset_start = cmp::min(offset, len);
  let offset_end = cmp::min(offset+size, len);
  
  chunk[offset_start..offset_end].to_vec()
}