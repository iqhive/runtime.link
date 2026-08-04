package snow

// FlakeID identifier to a value of type T.
// https://en.wikipedia.org/wiki/Snowflake_ID
type FlakeID[T any] int64
