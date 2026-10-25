package manager

func (m *Manager) cloneState() State {
	s := m.state
	s.Repositories = append([]Repository{}, s.Repositories...)
	s.Installations = append([]Installation{}, s.Installations...)
	s.Bindings = append([]Binding{}, s.Bindings...)
	return s
}
