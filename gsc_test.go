package sprites

import (
	"os"
	"testing"
)

func BenchmarkSprites(B *testing.B) {
	f, err := os.Open("games/en/crystal.gbc")
	if err != nil {
		B.Skip("couldn't open crystal.gbc:", err)
	}
	defer f.Close()
	rip, err := NewRipper(f)
	if err != nil {
		B.Fatal("couldn't open ripper:", err)
	}
	for i := 0; i < B.N; i++ {
		for n := 1; n <= 251; n++ {
			_, err := rip.Pokemon(n)
			if err != nil {
				B.Errorf("error while ripping sprite %d: %s", n, err)
			}
		}
	}
}

func BenchmarkAnimations(B *testing.B) {
	f, err := os.Open("games/en/crystal.gbc")
	if err != nil {
		B.Skip("couldn't open crystal.gbc:", err)
	}
	defer f.Close()
	rip, err := NewRipper(f)
	if err != nil {
		B.Fatal("couldn't open ripper:", err)
	}
	for i := 0; i < B.N; i++ {
		for n := 1; n <= 251; n++ {
			if n == 201 {
				continue
			}
			_, err := rip.PokemonAnimation(n)
			if err != nil {
				B.Errorf("error while ripping animation %d: %s", n, err)
			}
		}
	}
}
