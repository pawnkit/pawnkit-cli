package cli

import "github.com/pawnkit/pawn-project/dependency"

func cachedGitInstaller() dependency.GitInstaller {
	cacheDir, err := dependency.DefaultDependencyCacheDir()
	if err != nil {
		return dependency.GitInstaller{}
	}
	return dependency.GitInstaller{CacheDir: cacheDir}
}
