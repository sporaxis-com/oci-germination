package launcher

import "path/filepath"

func PostgresArgs(pgData string, preload string) []string {
	args := []string{
		filepath.Join("/usr/lib/postgresql", "17", "bin", "postgres"),
		"-D", pgData,
	}

	if preload != "" {
		args = append(args, "-c", "shared_preload_libraries="+preload)
	}

	return args
}
