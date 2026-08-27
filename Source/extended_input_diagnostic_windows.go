//go:build windows

package main

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func extendedInputMeaningfulChange(a, b dualSenseExtendedInput) bool {
	if a.ReportID != b.ReportID || a.ReportSize != b.ReportSize || a.Transport != b.Transport ||
		a.Create != b.Create || a.PS != b.PS || a.Mute != b.Mute || a.TouchClick != b.TouchClick ||
		a.EdgeFn1 != b.EdgeFn1 || a.EdgeFn2 != b.EdgeFn2 || a.EdgeLeft != b.EdgeLeft || a.EdgeRight != b.EdgeRight {
		return true
	}
	for i := 0; i < 2; i++ {
		if a.Touch[i].Active != b.Touch[i].Active || a.Touch[i].ID != b.Touch[i].ID {
			return true
		}
		if a.Touch[i].Active && (extAbsInt(a.Touch[i].X-b.Touch[i].X) >= 8 || extAbsInt(a.Touch[i].Y-b.Touch[i].Y) >= 8) {
			return true
		}
	}
	return false
}

func extAbsInt(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

func printExtendedInputState(s dualSenseExtendedInput, g logicalTouchFrame) {
	fmt.Printf("report=0x%02X len=%d mode=%s | Create=%t PS=%t Mute=%t TouchClick=%t | Edge Fn1=%t Fn2=%t L=%t R=%t\n",
		s.ReportID, s.ReportSize, s.Transport, s.Create, s.PS, s.Mute, s.TouchClick,
		s.EdgeFn1, s.EdgeFn2, s.EdgeLeft, s.EdgeRight)

	for i, p := range s.Touch {
		if p.Active {
			fmt.Printf("  RAW slot%d id=%d x=%d(%.3f) y=%d(%.3f)\n", i+1, p.ID, p.X, touchNormX(p.X), p.Y, touchNormY(p.Y))
		} else {
			fmt.Printf("  RAW slot%d off\n", i+1)
		}
	}

	fmt.Printf("  logical contacts=%d tap1=%t tap2=%t\n", g.Count, g.OneTap, g.TwoTap)
	if g.PrimaryActive {
		fmt.Printf("  T1 id=%d abs=(%.3f,%.3f)\n", g.PrimaryID, g.Abs1X, g.Abs1Y)
	} else {
		fmt.Println("  T1 off")
	}
	if g.SecondaryActive {
		fmt.Printf("  T2 id=%d abs=(%.3f,%.3f)\n", g.SecondaryID, g.Abs2X, g.Abs2Y)
	} else {
		fmt.Println("  T2 off")
	}
	fmt.Printf("  motion one=(%+.3f,%+.3f) two=(%+.3f,%+.3f) pinch=%+.3f\n",
		g.OneDX, g.OneDY, g.TwoDX, g.TwoDY, g.Pinch)
	fmt.Printf("  BeamNG centered raw: one=(%.3f,%.3f) two=(%.3f,%.3f) pinch=%.3f\n",
		centeredAxisRaw(g.OneDX), centeredAxisRaw(g.OneDY), centeredAxisRaw(g.TwoDX), centeredAxisRaw(g.TwoDY), centeredAxisRaw(g.Pinch))
}

func centeredAxisRaw(v float64) float64 {
	if v > 1 {
		v = 1
	}
	if v < -1 {
		v = -1
	}
	return (v + 1) * 0.5
}

func runExtendedInputDiagnostic(d *device) int {
	if d == nil {
		return 2
	}
	fmt.Println("DualSense Extended Input diagnostic")
	fmt.Printf("Controller: %s | PID=0x%04X | input=%d output=%d\n", d.product, d.productID, d.inputLen, d.outputLen)
	fmt.Println("Test one/two-finger taps and slow/fast movement in all directions.")
	fmt.Println("The diagnostic shows raw absolute touch data and derived relative motion separately.")
	fmt.Println("Press Ctrl+C to stop. This diagnostic does not emulate keyboard/mouse and does not change BeamNG bindings.")

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(stop)

	var tracker touchGestureTracker
	var last dualSenseExtendedInput
	haveLast := false
	lastPrint := time.Time{}
	for {
		select {
		case <-stop:
			return 0
		default:
		}
		r, err := d.readReportOnce()
		if err != nil {
			fmt.Println("Read error:", err)
			return 3
		}
		s, err := decodeDualSenseExtendedInput(r)
		if err != nil {
			if len(r) > 0 {
				fmt.Printf("RAW report=0x%02X len=%d decode=%v\n", r[0], len(r), err)
			}
			continue
		}
		now := time.Now()
		g := tracker.Update(s.Touch, now)
		moving := g.OneDX != 0 || g.OneDY != 0 || g.TwoDX != 0 || g.TwoDY != 0 || g.Pinch != 0
		if !haveLast || extendedInputMeaningfulChange(last, s) || moving || g.OneTap || g.TwoTap || now.Sub(lastPrint) >= 2*time.Second {
			printExtendedInputState(s, g)
			last = s
			haveLast = true
			lastPrint = now
		}
	}
}
