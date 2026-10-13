package logtail

import "cod4monitor/internal/store"

func (t *Tail) recordRead(st *store.Store, file string, backlog int64, lines, rejected int) {
	st.LogHealthRead(t.ServerID, file, backlog, lines, rejected)
}
func (t *Tail) recordError(st *store.Store, file string) {
	st.LogHealthSignal(t.ServerID, file, "error")
}
func (t *Tail) recordRotation(st *store.Store, file string) {
	st.LogHealthSignal(t.ServerID, file, "rotation")
}
func (t *Tail) recordMissing(st *store.Store, file string) {
	st.LogHealthSignal(t.ServerID, file, "missing")
}
