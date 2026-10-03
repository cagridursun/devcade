package mazechase

import (
	"errors"
	"fmt"
)

// Map size in logical cells. Each cell is drawn two terminal columns wide.
const (
	Cols = 29
	Rows = 19

	numChasers = 4
	numPowers  = 4

	// minSpawnDistance is the smallest walking distance allowed between the
	// player spawn and any chaser spawn.
	minSpawnDistance = 6
)

// layout is DevCade's original Maze Chase level. Tiles:
//
//	#  wall              .  pellet (10 points)
//	o  power pellet      ' ' empty floor
//	P  player spawn      1-4 chaser spawns (empty floor)
//
// The outer ring is solid wall, so a step never leaves the map, and the map
// has no dead ends. The chaser room in the middle is open on both sides.
var layout = [Rows]string{
	"#############################",
	"#o............#............o#",
	"#.###.####.##.#.##.####.###.#",
	"#...........................#",
	"#.###.#.#####.#.#####.#.###.#",
	"#.....#.......#.......#.....#",
	"#####.####.#######.####.#####",
	"#..........#######..........#",
	"#.###.####. 12 34 .####.###.#",
	"#..........#######..........#",
	"#####.####.#######.####.#####",
	"#.....#.......P.......#.....#",
	"#.###.#.#####.#.#####.#.###.#",
	"#...........................#",
	"#.###.####.#######.####.###.#",
	"#...........................#",
	"#.########.###.###.########.#",
	"#o.........................o#",
	"#############################",
}

type item uint8

const (
	none item = iota
	pellet
	power
)

// mazeInfo is the parsed, validated level.
type mazeInfo struct {
	wall    [Rows][Cols]bool
	items   [Rows][Cols]item
	player  point
	chasers [numChasers]point
	pellets int // ordinary pellets
	powers  int // power pellets
}

// maze is parsed once; an invalid built-in map is a programming error.
var maze = mustParse(layout)

func mustParse(rows [Rows]string) mazeInfo {
	m, err := parseMaze(rows[:])
	if err != nil {
		panic("mazechase: invalid built-in map: " + err.Error())
	}
	return m
}

// parseMaze parses and validates a map: exact Cols x Rows rectangle, legal
// tiles, a solid outer wall, one player spawn, one spawn per chaser, exactly
// four power pellets, every open cell (so every collectible) reachable from
// the player spawn, chaser spawns at least minSpawnDistance steps away from
// the player, and every fixed chaser target on open floor.
func parseMaze(rows []string) (mazeInfo, error) {
	var m mazeInfo
	if len(rows) != Rows {
		return m, fmt.Errorf("map has %d rows, want %d", len(rows), Rows)
	}
	player := 0
	var seen [numChasers]bool
	for y, row := range rows {
		if len(row) != Cols {
			return m, fmt.Errorf("row %d has %d columns, want %d", y, len(row), Cols)
		}
		for x := range Cols {
			tile := row[x]
			edge := x == 0 || y == 0 || x == Cols-1 || y == Rows-1
			if edge && tile != '#' {
				return m, fmt.Errorf("outer wall open at %d,%d", x, y)
			}
			switch tile {
			case '#':
				m.wall[y][x] = true
			case '.':
				m.items[y][x] = pellet
				m.pellets++
			case 'o':
				m.items[y][x] = power
				m.powers++
			case ' ':
			case 'P':
				m.player = point{x, y}
				player++
			case '1', '2', '3', '4':
				i := int(tile - '1')
				if seen[i] {
					return m, fmt.Errorf("duplicate chaser spawn %c", tile)
				}
				seen[i] = true
				m.chasers[i] = point{x, y}
			default:
				return m, fmt.Errorf("illegal tile %q at %d,%d", tile, x, y)
			}
		}
	}
	if player != 1 {
		return m, fmt.Errorf("found %d player spawns, want 1", player)
	}
	for i, ok := range seen {
		if !ok {
			return m, fmt.Errorf("missing chaser spawn %d", i+1)
		}
	}
	if m.powers != numPowers {
		return m, fmt.Errorf("found %d power pellets, want %d", m.powers, numPowers)
	}
	if m.pellets == 0 {
		return m, errors.New("no pellets")
	}
	dist := m.distances(m.player)
	for y := range Rows {
		for x := range Cols {
			if m.wall[y][x] && m.items[y][x] != none {
				return m, fmt.Errorf("collectible inside a wall at %d,%d", x, y)
			}
			if !m.wall[y][x] && dist[y][x] < 0 {
				return m, fmt.Errorf("open cell %d,%d unreachable from the player spawn", x, y)
			}
		}
	}
	for i, s := range m.chasers {
		if d := dist[s.y][s.x]; d < minSpawnDistance {
			return m, fmt.Errorf("chaser %d spawns %d steps from the player, want >= %d", i+1, d, minSpawnDistance)
		}
	}
	for _, p := range append(patrolRoute[:], shyCorner) {
		if !m.open(p) {
			return m, fmt.Errorf("chaser target %v is not open floor", p)
		}
	}
	return m, nil
}

func (m *mazeInfo) open(p point) bool {
	return p.x >= 0 && p.y >= 0 && p.x < Cols && p.y < Rows && !m.wall[p.y][p.x]
}

// distances returns the four-neighbor walking distance from `from` to every
// cell, or -1 for walls and unreachable cells.
func (m *mazeInfo) distances(from point) [Rows][Cols]int {
	var d [Rows][Cols]int
	for y := range d {
		for x := range d[y] {
			d[y][x] = -1
		}
	}
	if !m.open(from) {
		return d
	}
	queue := make([]point, 0, Rows*Cols)
	queue = append(queue, from)
	d[from.y][from.x] = 0
	for len(queue) > 0 {
		p := queue[0]
		queue = queue[1:]
		for _, dir := range dirOrder {
			n := p.add(dir)
			if m.open(n) && d[n.y][n.x] < 0 {
				d[n.y][n.x] = d[p.y][p.x] + 1
				queue = append(queue, n)
			}
		}
	}
	return d
}

// nearestOpen clamps p to the map and returns the open cell with the smallest
// Manhattan distance to it, preferring the first in row-major order on ties.
// The map is connected, so the result is always a reachable target.
func (m *mazeInfo) nearestOpen(p point) point {
	p.x = min(max(p.x, 0), Cols-1)
	p.y = min(max(p.y, 0), Rows-1)
	if m.open(p) {
		return p
	}
	best, bestD := point{}, -1
	for y := range Rows {
		for x := range Cols {
			if m.wall[y][x] {
				continue
			}
			d := abs(x-p.x) + abs(y-p.y)
			if bestD < 0 || d < bestD {
				best, bestD = point{x, y}, d
			}
		}
	}
	return best
}

func abs(v int) int {
	if v < 0 {
		return -v
	}
	return v
}
