import { useMemo, useState } from "react";

import { formatZagrebTime } from "../utils/time";
import type { DelayEvent } from "../types";

type DelaySort = "abs_delay" | "delay_seconds" | "broj_linije" | "scheduled_time" | "actual_time";

interface DelayBoardProps {
  delays: DelayEvent[];
  stale: boolean;
}

function sortDelays(delays: DelayEvent[], sort: DelaySort): DelayEvent[] {
  const rows = [...delays];

  rows.sort((left, right) => {
    switch (sort) {
      case "delay_seconds":
        return right.delay_seconds - left.delay_seconds;
      case "broj_linije":
        return left.broj_linije.localeCompare(right.broj_linije, "hr");
      case "scheduled_time":
        return right.scheduled_time.localeCompare(left.scheduled_time);
      case "actual_time":
        return right.actual_time.localeCompare(left.actual_time);
      case "abs_delay":
      default:
        return Math.abs(right.delay_seconds) - Math.abs(left.delay_seconds);
    }
  });

  return rows;
}

export function DelayBoard({ delays, stale }: DelayBoardProps): JSX.Element {
  const [sort, setSort] = useState<DelaySort>("abs_delay");
  const sortedDelays = useMemo(() => sortDelays(delays, sort), [delays, sort]);

  return (
    <section className={`panel board-panel${stale ? " panel--stale" : ""}`} data-testid="delay-board">
      <header className="panel-header board-header">
        <div>
          <h2>Delay Board</h2>
          <p>{sortedDelays.length} tracked delays</p>
        </div>
        <label className="sort-control" htmlFor="delay-sort">
          Sort
          <select id="delay-sort" value={sort} onChange={(event) => setSort(event.target.value as DelaySort)}>
            <option value="abs_delay">Absolute delay</option>
            <option value="delay_seconds">Delay seconds</option>
            <option value="broj_linije">Line</option>
            <option value="scheduled_time">Scheduled</option>
            <option value="actual_time">Actual</option>
          </select>
        </label>
      </header>

      <div className="board-table-wrap">
        <table>
          <thead>
            <tr>
              <th>Line</th>
              <th>Station</th>
              <th>Delay (s)</th>
              <th>Scheduled (Europe/Zagreb)</th>
              <th>Actual (Europe/Zagreb)</th>
            </tr>
          </thead>
          <tbody>
            {sortedDelays.map((delay) => (
              <tr key={`${delay.voznja_bus_id}-${delay.station_id}`}>
                <td>{delay.broj_linije || "-"}</td>
                <td>{delay.station_name || "-"}</td>
                <td className={delay.delay_seconds > 0 ? "delay-positive" : "delay-early"}>{delay.delay_seconds}</td>
                <td>{formatZagrebTime(delay.scheduled_time)}</td>
                <td>{formatZagrebTime(delay.actual_time)}</td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </section>
  );
}
