import SwiftUI

struct StatsView: View {
    let stats: StatsResponse

    var body: some View {
        HStack(spacing: 8) {
            StatCard(value: "\(stats.thisWeek)", label: "this week")
            StatCard(value: "\(stats.streak)", label: "day strk")
            StatCard(value: "\(stats.totalSessions)", label: "total sess")
            SparklineCard(activity: stats.activity)
        }
    }
}

private struct StatCard: View {
    let value: String
    let label: String

    var body: some View {
        VStack(spacing: 2) {
            Text(value)
                .font(.title2.bold())
            Text(label)
                .font(.caption2)
                .foregroundStyle(.secondary)
        }
        .frame(maxWidth: .infinity)
        .padding(.vertical, 8)
        .background(.quaternary.opacity(0.5))
        .clipShape(RoundedRectangle(cornerRadius: 8))
    }
}

private struct SparklineCard: View {
    let activity: [ActivityDay]

    var body: some View {
        VStack(spacing: 2) {
            SparklineView(values: activity.suffix(28).map(\.count))
                .frame(height: 24)
            Text("4 weeks")
                .font(.caption2)
                .foregroundStyle(.secondary)
        }
        .frame(maxWidth: .infinity)
        .padding(.vertical, 8)
        .background(.quaternary.opacity(0.5))
        .clipShape(RoundedRectangle(cornerRadius: 8))
    }
}

private struct SparklineView: View {
    let values: [Int]

    var body: some View {
        GeometryReader { geo in
            let maxVal = max(values.max() ?? 1, 1)
            let barWidth = max(geo.size.width / CGFloat(max(values.count, 1)) - 1, 2)

            HStack(alignment: .bottom, spacing: 1) {
                ForEach(Array(values.enumerated()), id: \.offset) { _, value in
                    RoundedRectangle(cornerRadius: 1)
                        .fill(.primary.opacity(0.6))
                        .frame(
                            width: barWidth,
                            height: max(CGFloat(value) / CGFloat(maxVal) * geo.size.height, 2)
                        )
                }
            }
        }
        .padding(.horizontal, 4)
    }
}
