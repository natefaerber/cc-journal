import SwiftUI

struct EntryListView: View {
    let entries: [JournalEntry]

    var body: some View {
        VStack(alignment: .leading, spacing: 8) {
            Text("Today (\(entries.count) sessions)")
                .font(.subheadline.bold())
                .foregroundStyle(.secondary)

            if entries.isEmpty {
                Text("No sessions yet today")
                    .font(.callout)
                    .foregroundStyle(.tertiary)
                    .frame(maxWidth: .infinity, alignment: .center)
                    .padding(.vertical, 12)
            } else {
                VStack(spacing: 0) {
                    ForEach(Array(entries.enumerated()), id: \.element.id) { index, entry in
                        EntryRow(entry: entry)
                        if index < entries.count - 1 {
                            Divider()
                        }
                    }
                }
                .background(.quaternary.opacity(0.3))
                .clipShape(RoundedRectangle(cornerRadius: 8))
            }
        }
    }
}
