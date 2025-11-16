package dom

type StorageType string

const (
	StorageTypeSource      StorageType = "source"
	StorageTypeDestination StorageType = "destination"
	StorageTypeBoth        StorageType = "both"
)

func (t StorageType) Valid() bool {
	switch t {
	case StorageTypeSource, StorageTypeDestination, StorageTypeBoth:
		return true
	default:
		return false
	}
}
