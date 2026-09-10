//go:build mcp

package main

func (s *MicrosoftAccountService) GetLoginDiagnostics() map[string]any {
	s.mu.Lock()
	host := s.host
	s.mu.Unlock()
	if host == nil {
		return map[string]any{"host": false}
	}
	return host.DebugSnapshot()
}

func (s *MicrosoftAccountService) GetLoginPageMetrics() map[string]any {
	s.mu.Lock()
	host := s.host
	s.mu.Unlock()
	if host == nil {
		return map[string]any{"host": false}
	}
	return host.DebugPageMetrics()
}
