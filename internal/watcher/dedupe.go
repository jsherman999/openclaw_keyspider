package watcher

// seenRecently maintains an in-memory ring buffer of recent hashes per host.
func (w *Watcher) seenRecently(hostID int64, sha string) bool {
	w.mu.Lock()
	defer w.mu.Unlock()

	win := w.cfg.Watcher.DedupeWindow
	if win <= 0 {
		win = 256
	}

	buf := w.recent[hostID]
	if len(buf) == 0 {
		buf = make([]string, win)
		w.recent[hostID] = buf
		w.recentI[hostID] = 0
	}

	for _, v := range buf {
		if v == sha {
			return true
		}
	}

	i := w.recentI[hostID]
	buf[i%len(buf)] = sha
	w.recentI[hostID] = (i + 1) % len(buf)
	w.recent[hostID] = buf
	return false
}
