package main

import "testing"

func putTouch(dst []byte, active bool, id byte, x, y int) {
	contact := id & 0x7f
	if !active {
		contact |= 0x80
	}
	dst[0] = contact
	dst[1] = byte(x)
	dst[2] = byte((x>>8)&0x0f) | byte((y&0x0f)<<4)
	dst[3] = byte(y >> 4)
}

func TestDecodeDualSenseExtendedUSB(t *testing.T) {
	r := make([]byte, 64)
	r[0] = 0x01
	common := 1
	r[common+8] = 0x10 // Create
	r[common+9] = 0x01 | 0x02 | 0x04 | 0x10 | 0x40
	putTouch(r[common+32:common+36], true, 3, 1919, 1079)
	putTouch(r[common+36:common+40], false, 4, 123, 456)
	s, err := decodeDualSenseExtendedInput(r)
	if err != nil {
		t.Fatal(err)
	}
	if !s.Create || !s.PS || !s.Mute || !s.TouchClick || !s.EdgeFn1 || !s.EdgeLeft {
		t.Fatalf("buttons not decoded: %+v", s)
	}
	if !s.Touch[0].Active || s.Touch[0].X != 1919 || s.Touch[0].Y != 1079 {
		t.Fatalf("touch1=%+v", s.Touch[0])
	}
	if s.Touch[1].Active || s.Touch[1].X != 123 || s.Touch[1].Y != 456 {
		t.Fatalf("touch2=%+v", s.Touch[1])
	}
}

func TestDecodeDualSenseExtendedBluetooth(t *testing.T) {
	r := make([]byte, 78)
	r[0] = 0x31
	common := 2
	r[common+8] = 0x10
	r[common+9] = 0x24 // Mute + Fn2
	putTouch(r[common+32:common+36], true, 7, 960, 540)
	putTouch(r[common+36:common+40], true, 8, 100, 200)
	s, err := decodeDualSenseExtendedInput(r)
	if err != nil {
		t.Fatal(err)
	}
	if s.Transport != "Bluetooth enhanced" || !s.Create || !s.Mute || !s.EdgeFn2 {
		t.Fatalf("state=%+v", s)
	}
	if s.Touch[0].X != 960 || s.Touch[0].Y != 540 {
		t.Fatalf("touch=%+v", s.Touch[0])
	}
}

func TestRejectCompactBluetooth(t *testing.T) {
	r := make([]byte, 10)
	r[0] = 0x01
	if _, err := decodeDualSenseExtendedInput(r); err == nil {
		t.Fatal("expected compact BT report rejection")
	}
}
