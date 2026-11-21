package bucket

import "github.com/dgrslabs/bucket/pkg/bucketerr"

// Sentinel errors returned by Client methods. They are re-exported from
// pkg/bucketerr so callers can branch on failures with errors.Is without an extra
// import:
//
//	if errors.Is(err, bucket.ErrNotFound) { ... }
//
// Because these are the same values as bucketerr.ErrNotFound etc., either package
// works with errors.Is. See pkg/bucketerr for the full documentation and
// suggested HTTP status mappings.
var (
	ErrNotFound           = bucketerr.ErrNotFound
	ErrInvalidID          = bucketerr.ErrInvalidID
	ErrInvalidName        = bucketerr.ErrInvalidName
	ErrAlreadyExists      = bucketerr.ErrAlreadyExists
	ErrFileTooLarge       = bucketerr.ErrFileTooLarge
	ErrPathTraversal      = bucketerr.ErrPathTraversal
	ErrTrashBatchExceeded = bucketerr.ErrTrashBatchExceeded
	ErrBackendUnavailable = bucketerr.ErrBackendUnavailable
	ErrUploadRejected     = bucketerr.ErrUploadRejected
	ErrUnsupported        = bucketerr.ErrUnsupported
)
