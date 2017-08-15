

pub fn cut_chunk(chunk: Vec<u8>, offset: u64, size: u64) -> Vec<u8> {
  trace!("Cutting Chunk to {} - {}", offset, (offset+size));
  chunk.into_iter().skip(offset as usize).take(size as usize).collect()
}