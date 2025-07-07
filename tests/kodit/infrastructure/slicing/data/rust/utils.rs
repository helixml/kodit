/// Utility functions for the example project.

use std::fmt;

/// Error type for validation failures
#[derive(Debug)]
pub struct ValidationError {
    message: String,
}

impl fmt::Display for ValidationError {
    fn fmt(&self, f: &mut fmt::Formatter) -> fmt::Result {
        write!(f, "{}", self.message)
    }
}

impl std::error::Error for ValidationError {}

/// Validate that a value is positive
pub fn validate_positive(value: f64, name: &str) -> Result<(), ValidationError> {
    if value <= 0.0 {
        Err(ValidationError {
            message: format!("{} must be positive, got {}", name, value),
        })
    } else {
        Ok(())
    }
}

/// Calculate Euclidean distance between two points
pub fn calculate_distance(x1: f64, y1: f64, x2: f64, y2: f64) -> f64 {
    let dx = x2 - x1;
    let dy = y2 - y1;
    (dx * dx + dy * dy).sqrt()
}

/// Format an amount as currency
pub fn format_currency(amount: f64, currency: &str) -> String {
    format!("{:.2} {}", amount, currency)
}

/// Split a vector into chunks of specified size
pub fn chunk_vec<T: Clone>(items: &[T], chunk_size: usize) -> Result<Vec<Vec<T>>, ValidationError> {
    validate_positive(chunk_size as f64, "chunk_size")?;
    
    Ok(items
        .chunks(chunk_size)
        .map(|chunk| chunk.to_vec())
        .collect())
}