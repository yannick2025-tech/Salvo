//go:build windows

package runner

// sampleSystem fills in Windows system metrics. On Windows, only Go
// runtime metrics are available; system metrics are set to zero.
func (c *RuntimeMetricsCollector) sampleSystem(s *RuntimeMetricsSnapshot) {
	s.CPUUsagePercent = 0
	s.RSSMemoryMB = 0x
	s.ThreadCount = 0
}
