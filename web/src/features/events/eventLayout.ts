export type LaneSlot = {
  start: number
  end: number
}

export type Lane = {
  col: number
  cols: number
}

export function layoutLanes(slots: LaneSlot[]): Lane[] {
  const n = slots.length
  const lanes: Lane[] = Array.from({ length: n }, () => ({ col: 0, cols: 1 }))
  if (n === 0) return lanes

  const order = slots.map((_, i) => i).sort((a, b) => {
    const start = slots[a].start - slots[b].start
    if (start !== 0) return start
    const dur = slots[b].end - slots[b].start - (slots[a].end - slots[a].start)
    if (dur !== 0) return dur
    return a - b
  })

  let clusterFrom = 0
  let clusterEnd = Number.NEGATIVE_INFINITY
  const colEnds: number[] = []

  function flush(to: number) {
    const cols = Math.max(1, colEnds.length)
    for (let k = clusterFrom; k < to; k++) lanes[order[k]].cols = cols
    colEnds.length = 0
  }

  for (let k = 0; k < order.length; k++) {
    const i = order[k]
    const slot = slots[i]
    if (k > clusterFrom && slot.start >= clusterEnd) {
      flush(k)
      clusterFrom = k
      clusterEnd = Number.NEGATIVE_INFINITY
    }
    let col = colEnds.findIndex((end) => end <= slot.start)
    if (col < 0) {
      col = colEnds.length
      colEnds.push(slot.end)
    } else {
      colEnds[col] = slot.end
    }
    lanes[i].col = col
    clusterEnd = Math.max(clusterEnd, slot.end)
  }
  flush(order.length)
  return lanes
}
