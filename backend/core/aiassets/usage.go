package aiassets

// packageUsage resolves algorithm ownership against the application's model
// contracts. A compatible alternative may serve several capabilities (PPE),
// and a dependency may support others without implementing them (pose/fighting).
func (s *Service) packageUsage(p Package, releases map[string]map[string]Package) PackageUsage {
	usage := PackageUsage{PackageID: p.ID, Version: p.Version, Capabilities: []string{}, RequiredBy: []string{}}
	for _, id := range sortedKeys(s.capabilities) {
		capability := s.capabilities[id]
		if capability.Provider != p.Provider || capability.UnsupportedReason != "" {
			continue
		}
		direct, dependency := false, false
		seenPackages := map[string]bool{}
		var visitPackage func(string, bool)
		visitPackage = func(pid string, isDirect bool) {
			// Test direct ownership before deduplication: a shared dependency can
			// also be a direct resource of the same capability.
			for _, candidate := range releases[pid] {
				if compatibleRelease(p, candidate) {
					if isDirect {
						direct = true
					} else {
						dependency = true
					}
				}
			}
			if seenPackages[pid] {
				return
			}
			seenPackages[pid] = true
			for _, candidate := range releases[pid] {
				for _, dep := range candidate.Dependencies {
					visitPackage(dep, false)
				}
			}
		}
		seenCapabilities := map[string]bool{}
		var visitCapability func(string, bool)
		visitCapability = func(cid string, isDirect bool) {
			if seenCapabilities[cid] {
				return
			}
			seenCapabilities[cid] = true
			c, ok := s.capabilities[cid]
			if !ok || c.UnsupportedReason != "" || c.Provider != p.Provider {
				return
			}
			for _, pid := range c.Packages {
				visitPackage(pid, isDirect)
			}
			for _, dep := range c.Dependencies {
				visitCapability(dep, false)
			}
		}
		visitCapability(id, true)
		if direct {
			usage.Capabilities = append(usage.Capabilities, id)
		} else if dependency {
			usage.RequiredBy = append(usage.RequiredBy, id)
		}
	}
	return usage
}

func (s *Service) planUsageLocked(plan Plan) []PackageUsage {
	result := make([]PackageUsage, 0, len(plan.Packages))
	for _, p := range plan.Packages {
		usage := s.packageUsage(p, s.packages)
		usage.Effect = "activate_if_missing"
		if s.active.Packages[p.ID] == p.Version && s.installed(p) {
			usage.Effect = "reuse_active"
		} else if !plan.Offline {
			// An offline import is a vendor delivery or upgrade and takes over its
			// resource kind, so only an online plan can leave a package as candidate.
			if legacy := s.providers[p.Provider].Legacy; legacy != nil && legacy(s.root, p.Kind) {
				usage.Effect = "candidate"
			}
			for id, version := range s.active.Packages {
				if s.packages[id][version].Kind == p.Kind {
					usage.Effect = "candidate"
					break
				}
			}
		}
		result = append(result, usage)
	}
	return result
}
