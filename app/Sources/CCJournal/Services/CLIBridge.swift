import Foundation

struct CLIBridge: Sendable {
    private let binaryPath: String

    init(binaryPath: String? = nil) {
        self.binaryPath = binaryPath ?? Self.findBinary()
    }

    func fetchStats() async throws -> StatsResponse {
        try await run(["stats", "--json"])
    }

    func fetchToday() async throws -> EntriesResponse {
        try await run(["today", "--json"])
    }

    func fetchEntries(date: String) async throws -> EntriesResponse {
        try await run(["show", date, "--json"])
    }

    func fetchFileList() async throws -> FileListResponse {
        try await run(["list", "--json"])
    }

    func copyStandup() async throws {
        _ = try await runRaw(["standup", "--copy"])
    }

    func copyWeekly() async throws {
        _ = try await runRaw(["weekly", "--copy"])
    }

    // MARK: - Private

    private func run<T: Decodable>(_ arguments: [String]) async throws -> T {
        let data = try await runRaw(arguments)
        return try JSONDecoder().decode(T.self, from: data)
    }

    private func runRaw(_ arguments: [String]) async throws -> Data {
        try await withCheckedThrowingContinuation { continuation in
            DispatchQueue.global(qos: .userInitiated).async {
                do {
                    let process = Process()
                    process.executableURL = URL(fileURLWithPath: binaryPath)
                    process.arguments = arguments

                    let pipe = Pipe()
                    let errorPipe = Pipe()
                    process.standardOutput = pipe
                    process.standardError = errorPipe

                    try process.run()
                    process.waitUntilExit()

                    guard process.terminationStatus == 0 else {
                        let stderr = String(data: errorPipe.fileHandleForReading.readDataToEndOfFile(), encoding: .utf8) ?? "Unknown error"
                        continuation.resume(throwing: CLIError.nonZeroExit(Int(process.terminationStatus), stderr))
                        return
                    }

                    let data = pipe.fileHandleForReading.readDataToEndOfFile()
                    continuation.resume(returning: data)
                } catch {
                    continuation.resume(throwing: error)
                }
            }
        }
    }

    /// Resolved path to the cc-journal binary, for sharing with ServerManager.
    static var resolvedBinaryPath: String { findBinary() }

    private static func findBinary() -> String {
        let candidates = [
            "/opt/homebrew/bin/cc-journal",
            "/usr/local/bin/cc-journal",
        ]

        // Check PATH first
        if let pathBinary = findInPath("cc-journal") {
            return pathBinary
        }

        for candidate in candidates {
            if FileManager.default.isExecutableFile(atPath: candidate) {
                return candidate
            }
        }

        return "cc-journal" // fallback — will fail with a clear error
    }

    private static func findInPath(_ binary: String) -> String? {
        guard let path = ProcessInfo.processInfo.environment["PATH"] else { return nil }
        for dir in path.split(separator: ":") {
            let full = "\(dir)/\(binary)"
            if FileManager.default.isExecutableFile(atPath: full) {
                return full
            }
        }
        return nil
    }
}

enum CLIError: LocalizedError {
    case nonZeroExit(Int, String)

    var errorDescription: String? {
        switch self {
        case .nonZeroExit(let code, let message):
            return "cc-journal exited with code \(code): \(message)"
        }
    }
}
