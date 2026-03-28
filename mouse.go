package main

import (
	"math"
	"math/rand"
	"time"

	"github.com/go-vgo/robotgo"
	"golang.design/x/hotkey/mainthread"
)

// Point represents a screen coordinate
type Point struct {
	X, Y float64
}

// randGauss retorna um valor aproximadamente gaussiano usando soma de uniforms (CLT).
// mean=0, stddev aproximado = scale.
func randGauss(scale float64) float64 {
	sum := 0.0
	for i := 0; i < 6; i++ {
		sum += rand.Float64()
	}
	return (sum/6 - 0.5) * 2 * scale
}

// randRange retorna um float64 aleatório entre min e max.
func randRange(min, max float64) float64 {
	return min + rand.Float64()*(max-min)
}

// MoveHumanLike move o mouse até (targetX, targetY) simulando movimento humano:
// - Curva bezier cúbica com offset perpendicular ao trajeto (arco natural)
// - Perfil de velocidade não-uniforme (devagar-rápido-devagar com variações)
// - Micro-jitter gaussiano
// - Micro-pauses ocasionais (como hesitação humana)
// - Pequeno overshoot + correção ao final (comportamento natural de mão)
func MoveHumanLike(targetX, targetY int) {
	curX, curY := robotgo.GetMousePos()
	start := Point{float64(curX), float64(curY)}
	end := Point{float64(targetX), float64(targetY)}

	dx := end.X - start.X
	dy := end.Y - start.Y
	dist := math.Sqrt(dx*dx + dy*dy)
	if dist < 5 {
		mainthread.Call(func() { robotgo.Move(targetX, targetY) })
		return
	}

	// Vetor perpendicular ao trajeto (normalizado)
	perpX := -dy / dist
	perpY := dx / dist

	// Curvatura: offset perpendicular aleatório (sinal e magnitude)
	curveMag := randRange(0.08, 0.22) * dist
	if rand.Float64() < 0.5 {
		curveMag = -curveMag
	}

	// Pontos de controle da bezier cúbica com offset perpendicular
	t1 := randRange(0.20, 0.35)
	t2 := randRange(0.65, 0.80)
	cp1 := Point{
		X: start.X + dx*t1 + perpX*curveMag*randRange(0.7, 1.3),
		Y: start.Y + dy*t1 + perpY*curveMag*randRange(0.7, 1.3),
	}
	cp2 := Point{
		X: start.X + dx*t2 + perpX*curveMag*randRange(0.5, 1.0),
		Y: start.Y + dy*t2 + perpY*curveMag*randRange(0.5, 1.0),
	}

	steps := int(dist/8) + 12 + rand.Intn(10)

	// Micro-pause ocasional: 20% de chance, em algum ponto do meio
	pauseStep := -1
	if rand.Float64() < 0.20 {
		pauseStep = steps/4 + rand.Intn(steps/2)
	}

	for i := 0; i <= steps; i++ {
		t := float64(i) / float64(steps)

		// Perfil de velocidade: ease-in-out com variação (não perfeitamente simétrico)
		eased := t * t * (3 - 2*t)
		// Leve assimetria humana: humanos aceleram mais rápido do que desaceleram
		eased = eased*0.85 + t*0.15

		x := math.Pow(1-eased, 3)*start.X +
			3*math.Pow(1-eased, 2)*eased*cp1.X +
			3*(1-eased)*math.Pow(eased, 2)*cp2.X +
			math.Pow(eased, 3)*end.X
		y := math.Pow(1-eased, 3)*start.Y +
			3*math.Pow(1-eased, 2)*eased*cp1.Y +
			3*(1-eased)*math.Pow(eased, 2)*cp2.Y +
			math.Pow(eased, 3)*end.Y

		// Jitter gaussiano — maior no início/meio, quase zero no fim
		jitterScale := (1 - t) * 1.2
		jx := randGauss(jitterScale)
		jy := randGauss(jitterScale)

		mainthread.Call(func() {
			robotgo.Move(int(x+jx), int(y+jy))
		})

		// Micro-pause ocasional
		if i == pauseStep {
			time.Sleep(time.Duration(randRange(40, 120)) * time.Millisecond)
		}

		// Velocidade variável: mais lento no início e fim, mais rápido no meio
		speedFactor := 0.5 + math.Sin(t*math.Pi)*0.5 // 0→1→0
		delay := randRange(5, 14) * (1.5 - speedFactor)
		time.Sleep(time.Duration(delay) * time.Millisecond)
	}

	// Pequeno overshoot + correção (30% das vezes)
	if rand.Float64() < 0.30 {
		ovX := targetX + int(randGauss(4))
		ovY := targetY + int(randGauss(3))
		mainthread.Call(func() { robotgo.Move(ovX, ovY) })
		time.Sleep(time.Duration(randRange(20, 50)) * time.Millisecond)
	}

	// Move final preciso
	mainthread.Call(func() { robotgo.Move(targetX, targetY) })
	time.Sleep(time.Duration(randRange(60, 160)) * time.Millisecond)
}

// ClickHumanLike realiza um clique com duração e pausa pós-clique aleatórios.
func ClickHumanLike() {
	mainthread.Call(func() { robotgo.MouseDown("left") })
	time.Sleep(time.Duration(randRange(40, 130)) * time.Millisecond)
	mainthread.Call(func() { robotgo.MouseUp("left") })
	time.Sleep(time.Duration(randRange(80, 220)) * time.Millisecond)
}

// TypeHumanLike digita uma string simulando ritmo humano:
// - Delay base variável por caractere
// - Bursts: sequências rápidas seguidas de pequenas pausas
// - Pauses ocasionais mais longas (hesitação/pensamento)
// - Espaços e pontuação tendem a ser mais lentos
func TypeHumanLike(str string) {
	burstCount := 0
	burstLen := 2 + rand.Intn(4) // digita 2-5 chars rápido antes de uma pausa leve

	for _, char := range str {
		mainthread.Call(func() {
			robotgo.TypeStr(string(char))
		})

		var delay float64

		switch {
		case char == ' ':
			// Espaço: pausa um pouco maior (fim de palavra)
			delay = randRange(80, 200)
		case char >= 'A' && char <= 'Z':
			// Maiúsculas (shift pressionado): um pouco mais devagar
			delay = randRange(60, 160)
		default:
			delay = randRange(40, 140)
		}

		// Modo burst: alguns chars consecutivos mais rápidos
		burstCount++
		if burstCount < burstLen {
			delay *= randRange(0.4, 0.7)
		} else {
			// Fim do burst: pausa leve entre bursts
			delay += randRange(20, 80)
			burstCount = 0
			burstLen = 2 + rand.Intn(4)
		}

		// Pausa longa ocasional (3% de chance) — hesitação humana
		if rand.Float64() < 0.03 {
			delay += randRange(200, 600)
		}

		time.Sleep(time.Duration(delay) * time.Millisecond)
	}

	// Pausa após terminar de digitar
	time.Sleep(time.Duration(randRange(150, 350)) * time.Millisecond)
}
