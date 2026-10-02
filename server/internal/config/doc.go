// Package config holds the compile-time content output model shared between
// the content compiler (cmd/compiler) and the activation pipeline (IMP-004):
// CandidateSnapshot, canonical payload serialization, content revision, the
// diagnostic set, the field-source coverage report, and the compile report.
//
// It contains no compiler machinery and no activation lifecycle logic; it is
// the data contract both sides code against.
package config
