import Foundation
import LocalAuthentication

struct HelperRequest: Decodable {
    let id: String?
    let type: String
    let action: String?
    let reason: String?
}

struct HelperResponse: Encodable {
    let id: String?
    let type: String
    let status: String?
    let provider: String?
    let message: String?
}

final class HelperRuntime {
    private let encoder = JSONEncoder()
    private let writeQueue = DispatchQueue(label: "me.ritik.forged.auth.write")
    private var inputBuffer = Data()

    func run() {
        let center = DistributedNotificationCenter.default()
        center.addObserver(forName: NSNotification.Name("com.apple.screenIsLocked"), object: nil, queue: nil) { [weak self] _ in
            self?.emit(HelperResponse(id: nil, type: "event", status: "session_locked", provider: "local-authentication", message: nil))
        }
        center.addObserver(forName: NSNotification.Name("com.apple.screensaver.didstart"), object: nil, queue: nil) { [weak self] _ in
            self?.emit(HelperResponse(id: nil, type: "event", status: "session_locked", provider: "local-authentication", message: nil))
        }

        let stdinHandle = FileHandle.standardInput
        stdinHandle.readabilityHandler = { [weak self] handle in
            let data = handle.availableData
            if data.isEmpty {
                exit(0)
            }
            self?.ingest(data)
        }

        dispatchMain()
    }

    private func ingest(_ data: Data) {
        inputBuffer.append(data)
        while let newline = inputBuffer.firstIndex(of: 0x0A) {
            let line = inputBuffer[..<newline]
            inputBuffer.removeSubrange(...newline)
            guard !line.isEmpty else { continue }
            guard let req = try? JSONDecoder().decode(HelperRequest.self, from: Data(line)) else { continue }
            handle(req)
        }
    }

    private func handle(_ req: HelperRequest) {
        switch req.type {
        case "authorize":
            authorize(request: req)
        case "subscribe-locks":
            emit(HelperResponse(id: req.id, type: req.type, status: "ok", provider: "local-authentication", message: nil))
        case "status":
            let (status, message) = capabilityStatus()
            emit(HelperResponse(id: req.id, type: req.type, status: status, provider: "local-authentication", message: message))
        default:
            emit(HelperResponse(id: req.id, type: req.type, status: "failed", provider: "local-authentication", message: "unsupported request"))
        }
    }

    private func authorize(request: HelperRequest) {
        let context = LAContext()
        if #available(macOS 10.12.2, *) {
            context.touchIDAuthenticationAllowableReuseDuration = 0
        }
        var policyError: NSError?
        let policy: LAPolicy = .deviceOwnerAuthentication

        guard context.canEvaluatePolicy(policy, error: &policyError) else {
            let status = helperUnavailableStatus(policyError as? LAError)
            emit(HelperResponse(id: request.id, type: request.type, status: status, provider: "local-authentication", message: policyError?.localizedDescription))
            return
        }

        let reason = (request.reason?.isEmpty == false ? request.reason! : "Authenticate to continue")
        context.evaluatePolicy(policy, localizedReason: reason) { [weak self] success, error in
            guard let self else { return }
            if success {
                self.emit(HelperResponse(id: request.id, type: request.type, status: "ok", provider: "local-authentication", message: nil))
                return
            }

            if let laError = error as? LAError {
                switch laError.code {
                case .userCancel, .appCancel, .systemCancel:
                    self.emit(HelperResponse(id: request.id, type: request.type, status: "canceled", provider: "local-authentication", message: laError.localizedDescription))
                    return
                case .notInteractive:
                    self.emit(HelperResponse(id: request.id, type: request.type, status: "unavailable_by_environment", provider: "local-authentication", message: laError.localizedDescription))
                    return
                case .biometryNotAvailable, .biometryNotEnrolled, .biometryLockout, .passcodeNotSet:
                    self.emit(HelperResponse(id: request.id, type: request.type, status: "unavailable_by_platform", provider: "local-authentication", message: laError.localizedDescription))
                    return
                default:
                    break
                }
            }

            self.emit(HelperResponse(id: request.id, type: request.type, status: "failed", provider: "local-authentication", message: error?.localizedDescription))
        }
    }

    private func emit(_ response: HelperResponse) {
        writeQueue.async {
            guard let data = try? self.encoder.encode(response) else { return }
            FileHandle.standardOutput.write(data)
            FileHandle.standardOutput.write("\n".data(using: .utf8)!)
        }
    }

    private func capabilityStatus() -> (String, String?) {
        let context = LAContext()
        if #available(macOS 10.12.2, *) {
            context.touchIDAuthenticationAllowableReuseDuration = 0
        }
        var policyError: NSError?
        let policy: LAPolicy = .deviceOwnerAuthentication

        if context.canEvaluatePolicy(policy, error: &policyError) {
            return ("ok", nil)
        }

        return (helperUnavailableStatus(policyError as? LAError), policyError?.localizedDescription)
    }
}

let runtime = HelperRuntime()
runtime.run()

func helperUnavailableStatus(_ error: LAError?) -> String {
    guard let error else { return "unavailable_by_environment" }
    switch error.code {
    case .notInteractive:
        return "unavailable_by_environment"
    case .biometryNotAvailable, .biometryNotEnrolled, .biometryLockout, .passcodeNotSet:
        return "unavailable_by_platform"
    default:
        return "unavailable_by_environment"
    }
}
