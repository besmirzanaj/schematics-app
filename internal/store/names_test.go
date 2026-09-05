package store

import "testing"

func TestCleanDisplay(t *testing.T) {
	cases := []struct{ raw, want, cargo string }{
		{"147 [2000]", "147", "2000"},
		{"147 ii° serie [2004]", "147 II Serie", "2004"},
		{"156 [1998]", "156", "1998"},
		{"156 [2001] s.w", "156 SW", "2001"},
		{"156 crosswagon q4", "156 Crosswagon Q4", ""},
		{"coupe`  (2002)", "Coupe", "2002"},
		{"coupe`  [2002]", "Coupe", "2002"},
		{"accent (cm41) [2006]", "Accent", "2006"},
		{"atos (ac51h) [2003]", "Atos", "2003"},
		{"h-1 (2wd)", "H-1 2WD", ""},
		{"h-100 (kn7fp)", "H-100", ""},
		{"ix35 [2010] (4wd)", "IX35 4WD", "2010"},
		{"terracan  (m81) `02", "Terracan", ""},
		{"santa fe`[2006]", "Santa Fe", "2006"},
		{"sonica-sonata [2001]", "Sonica-Sonata", "2001"},
		{"brera [2006]", "Brera", "2006"},
		{"excel [1997]", "Excel", "1997"},
		{"i10 [2007]", "I10", "2007"},
		{"veloster [2011]", "Veloster", "2011"},
		{"h-1 [1997] 2wd", "H-1 2WD", "1997"},
		{"3 [2003]", "3", "2003"},
		{"6 [2008] s.w", "6 SW", "2008"},
		{"1304 4WD", "1304 4WD", ""},
		{"1310 break", "1310 Break", ""},
		{"9000", "9000", ""},
		{"1007 [2005]", "1007", "2005"},
		{"911 (997) turbo (4wd)", "911 Turbo 4WD", ""},
		{"a3 [2012] 3d. 4wd", "A3 3d 4WD", "2012"},
		{"focus c-max (2003)", "Focus C-Max", "2003"},
		{"golf v (k4) 2003", "Golf V 2003", ""},
		{"xjs", "XJS", ""},
		{"", "", ""},
	}
	for _, c := range cases {
		if got := CleanDisplay(c.raw); got != c.want {
			t.Errorf("CleanDisplay(%q) = %q, want %q", c.raw, got, c.want)
		}
		if got := CargoYear(c.raw); got != c.cargo {
			t.Errorf("CargoYear(%q) = %q, want %q", c.raw, got, c.cargo)
		}
	}
}