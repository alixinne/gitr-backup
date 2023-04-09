package constants

const BACKUP_PREFIX = "[backup]"
const IGNORE_PREFIX = "[ignore]"
const BACKUP_LABEL = "gitr-backup"
const PRIVATE_LABEL = "private"

type ContextKey int

const (
	DRY_RUN ContextKey = iota
)
