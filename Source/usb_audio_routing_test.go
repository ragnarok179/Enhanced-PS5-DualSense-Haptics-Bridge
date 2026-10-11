package main

import "testing"

func TestDualSenseDefaultPlaybackClassification(t *testing.T) {
	cases := []struct{name, id, label, haptic string; want bool}{
		{"matched haptic endpoint", "{A}", "Generic USB Audio", "{a}", true},
		{"controller named", "B", "Headphones (Wireless Controller)", "other", true},
		{"DualSense named", "B", "Haut-parleurs (DualSense)", "other", true},
		{"PS5 named", "B", "Speakers (PS5 Controller)", "other", true},
		{"ordinary headphones", "B", "Headphones (Realtek)", "other", false},
		{"ordinary speakers", "B", "Speakers (USB DAC)", "other", false},
		{"missing IDs", "", "", "", false},
		{"blank haptic ID", "", "HDMI Audio", "", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := isDualSenseDefaultPlayback(tc.id, tc.label, tc.haptic); got != tc.want {
				t.Fatalf("got %t; want %t",got,tc.want)
			}
		})
	}
}
