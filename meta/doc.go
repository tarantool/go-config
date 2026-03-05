// Package meta provides metadata types that describe the origin and version
// of configuration values.
//
// # Key Types
//
//   - [Info] — full metadata for a value: Key ([keypath.KeyPath]),
//     Source ([SourceInfo]), and Revision ([RevisionType]).
//   - [SourceInfo] — identifies where a value came from: Name (string)
//     and Type ([SourceType]).
//   - [SourceType] — enum for source classification: [UnknownSource],
//     [EnvDefaultSource], [StorageSource], [FileSource], [EnvSource],
//     [ModifiedSource].
//   - [RevisionType] — a string identifier for the configuration revision
//     (e.g., commit hash, timestamp). Empty when not applicable.
package meta
