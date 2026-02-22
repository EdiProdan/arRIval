import { useMemo, useState } from "react";

import { formatZagrebTime } from "../utils/time";
import type { ObservedDelay, PredictedDelay } from "../types";

type ObservedSort = "abs_delay" | "delay_seconds" | "broj_linije" | "scheduled_time" | "observed_time";
type PredictedSort = "abs_delay" | "predicted_delay_seconds" | "broj_linije" | "scheduled_time" | "predicted_time";

interface DelayBoardProps {
  observedDelays: ObservedDelay[];
  predictedDelays: PredictedDelay[];
  stale: boolean;
}

function sortObservedDelays(delays: ObservedDelay[], sort: ObservedSort): ObservedDelay[] {
  const rows = [...delays];

  rows.sort((left, right) => {
    switch (sort) {
      case "delay_seconds":
        return right.delay_seconds - left.delay_seconds;
      case "broj_linije":
        return left.broj_linije.localeCompare(right.broj_linije, "hr");
      case "scheduled_time":
        return right.scheduled_time.localeCompare(left.scheduled_time);
      case "observed_time":
        return right.observed_time.localeCompare(left.observed_time);
      case "abs_delay":
      default:
        return Math.abs(right.delay_seconds) - Math.abs(left.delay_seconds);
    }
  });

  return rows;
}

function sortPredictedDelays(delays: PredictedDelay[], sort: PredictedSort): PredictedDelay[] {
  const rows = [...delays];

  rows.sort((left, right) => {
    switch (sort) {
      case "predicted_delay_seconds":
        return right.predicted_delay_seconds - left.predicted_delay_seconds;
      case "broj_linije":
        return left.broj_linije.localeCompare(right.broj_linije, "hr");
      case "scheduled_time":
        return right.scheduled_time.localeCompare(left.scheduled_time);
      case "predicted_time":
        return right.predicted_time.localeCompare(left.predicted_time);
      case "abs_delay":
      default:
        return Math.abs(right.predicted_delay_seconds) - Math.abs(left.predicted_delay_seconds);
    }
  });

  return rows;
}

export function DelayBoard({ observedDelays, predictedDelays, stale }: DelayBoardProps): JSX.Element {
  const [observedSort, setObservedSort] = useState<ObservedSort>("abs_delay");
  const [predictedSort, setPredictedSort] = useState<PredictedSort>("abs_delay");

  const sortedObserved = useMemo(
    () => sortObservedDelays(observedDelays, observedSort),
    [observedDelays, observedSort]
  );
  const sortedPredicted = useMemo(
    () => sortPredictedDelays(predictedDelays, predictedSort),
    [predictedDelays, predictedSort]
  );

  return (
    <section className={`panel board-panel${stale ? " panel--stale" : ""}`} data-testid="delay-board">
      <header className="panel-header board-header">
        <div>
          <h2>Delay Board</h2>
          <p>{sortedObserved.length} observed / {sortedPredicted.length} predicted</p>
        </div>
      </header>

      <div className="board-split">
        <section className="board-section" data-testid="observed-board">
          <header className="board-section-header">
            <div>
              <h3>Observed Delays</h3>
              <p>{sortedObserved.length} tracked stops</p>
            </div>
            <label className="sort-control" htmlFor="observed-delay-sort">
              Sort
              <select
                id="observed-delay-sort"
                value={observedSort}
                onChange={(event) => setObservedSort(event.target.value as ObservedSort)}
              >
                <option value="abs_delay">Absolute delay</option>
                <option value="delay_seconds">Delay seconds</option>
                <option value="broj_linije">Line</option>
                <option value="scheduled_time">Scheduled</option>
                <option value="observed_time">Observed</option>
              </select>
            </label>
          </header>
          <div className="board-table-wrap board-table-wrap--split">
            <table>
              <thead>
                <tr>
                  <th>Line</th>
                  <th>Station</th>
                  <th>Seq</th>
                  <th>Delay (s)</th>
                  <th>Scheduled (Europe/Zagreb)</th>
                  <th>Observed (Europe/Zagreb)</th>
                </tr>
              </thead>
              <tbody>
                {sortedObserved.map((delay) => (
                  <tr key={`${delay.trip_id}-${delay.station_id}`}>
                    <td>{delay.broj_linije || "-"}</td>
                    <td>{delay.station_name || "-"}</td>
                    <td>{delay.station_seq}</td>
                    <td className={delay.delay_seconds > 0 ? "delay-positive" : "delay-early"}>{delay.delay_seconds}</td>
                    <td>{formatZagrebTime(delay.scheduled_time)}</td>
                    <td>{formatZagrebTime(delay.observed_time)}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </section>

        <section className="board-section" data-testid="predicted-board">
          <header className="board-section-header">
            <div>
              <h3>Predicted Delays</h3>
              <p>{sortedPredicted.length} upcoming stops</p>
            </div>
            <label className="sort-control" htmlFor="predicted-delay-sort">
              Sort
              <select
                id="predicted-delay-sort"
                value={predictedSort}
                onChange={(event) => setPredictedSort(event.target.value as PredictedSort)}
              >
                <option value="abs_delay">Absolute delay</option>
                <option value="predicted_delay_seconds">Delay seconds</option>
                <option value="broj_linije">Line</option>
                <option value="scheduled_time">Scheduled</option>
                <option value="predicted_time">Predicted</option>
              </select>
            </label>
          </header>
          <div className="board-table-wrap board-table-wrap--split">
            <table>
              <thead>
                <tr>
                  <th>Line</th>
                  <th>Station</th>
                  <th>Seq</th>
                  <th>Delay (s)</th>
                  <th>Scheduled (Europe/Zagreb)</th>
                  <th>Predicted (Europe/Zagreb)</th>
                </tr>
              </thead>
              <tbody>
                {sortedPredicted.map((delay) => (
                  <tr key={`${delay.trip_id}-${delay.station_id}`}>
                    <td>{delay.broj_linije || "-"}</td>
                    <td>{delay.station_name || "-"}</td>
                    <td>{delay.station_seq}</td>
                    <td className={delay.predicted_delay_seconds > 0 ? "delay-positive" : "delay-early"}>
                      {delay.predicted_delay_seconds}
                    </td>
                    <td>{formatZagrebTime(delay.scheduled_time)}</td>
                    <td>{formatZagrebTime(delay.predicted_time)}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </section>
      </div>
    </section>
  );
}
