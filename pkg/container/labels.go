package container

// Labels Pumba puts on every sidecar it creates, in the com.gaiaadm.pumba.*
// namespace. SkipLabel keeps sidecars out of chaos target selection; the
// others let `pumba status` attribute a leftover sidecar to a run and target.
const (
	SkipLabel   = "com.gaiaadm.pumba.skip"
	RunIDLabel  = "com.gaiaadm.pumba.run-id"
	TargetLabel = "com.gaiaadm.pumba.target"
	KindLabel   = "com.gaiaadm.pumba.kind"
)

// Sidecar kinds, the values of KindLabel.
const (
	SidecarKindNetem    = "netem"
	SidecarKindIPTables = "iptables"
	SidecarKindStress   = "stress"
	SidecarKindInspect  = "inspect"
)

// SidecarLabels returns the labels for a sidecar of kind acting on targetID.
// An empty runID leaves the sidecar without a run label (untagged).
func SidecarLabels(runID, targetID, kind string) map[string]string {
	labels := map[string]string{
		SkipLabel:   trueValue,
		TargetLabel: targetID,
		KindLabel:   kind,
	}
	if runID != "" {
		labels[RunIDLabel] = runID
	}
	return labels
}
