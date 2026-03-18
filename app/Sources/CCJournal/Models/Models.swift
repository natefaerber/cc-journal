import Foundation

// MARK: - Stats

struct StatsResponse: Codable, Sendable {
    let totalSessions: Int
    let totalDays: Int
    let totalProjects: Int
    let thisWeek: Int
    let streak: Int
    let mostActive: String
    let activity: [ActivityDay]

    enum CodingKeys: String, CodingKey {
        case totalSessions = "total_sessions"
        case totalDays = "total_days"
        case totalProjects = "total_projects"
        case thisWeek = "this_week"
        case streak
        case mostActive = "most_active"
        case activity
    }
}

struct ActivityDay: Codable, Sendable, Identifiable {
    let date: String
    let count: Int

    var id: String { date }
}

// MARK: - Entries

struct EntriesResponse: Codable, Sendable {
    let date: String
    let entries: [JournalEntry]
}

struct JournalEntry: Codable, Sendable, Identifiable {
    let project: String
    let branch: String
    let timeRange: String
    let sessionId: String
    let cwd: String
    let summary: String
    let hasAiSummary: Bool

    var id: String { sessionId }

    var resumeCommand: String {
        "cd \(cwd) && claude --resume \(sessionId)"
    }

    enum CodingKeys: String, CodingKey {
        case project, branch, summary, cwd
        case timeRange = "time_range"
        case sessionId = "session_id"
        case hasAiSummary = "has_ai_summary"
    }
}

// MARK: - File List

struct FileListResponse: Codable, Sendable {
    let files: [FileInfo]
}

struct FileInfo: Codable, Sendable, Identifiable {
    let date: String
    let size: Int
    let entryCount: Int

    var id: String { date }

    enum CodingKeys: String, CodingKey {
        case date, size
        case entryCount = "entry_count"
    }
}
