import AppKit
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
    var secret: String? = nil
}

private final class AuthorizationState {
    let context = LAContext()
}

private final class PasswordPromptState {
    var alert: NSAlert?
}

private enum ActiveRequest {
    case pending
    case authorization(AuthorizationState)
    case password(PasswordPromptState)
}

final class HelperRuntime {
    private let encoder = JSONEncoder()
    private let writeQueue = DispatchQueue(label: "me.ritik.forged.auth.write")
    private let stateLock = NSLock()
    private var activeRequests: [String: ActiveRequest] = [:]
    private var inputBuffer = Data()
    private var lastLockEvent = Date.distantPast

    // start wires up stdin and lock observers. The caller runs the NSApplication
    // event loop (below) rather than dispatchMain(): AppKit UI must run on the
    // real pthread main thread, which dispatchMain() parks away from GCD.
    func start() {
        // A screensaver alone is not a lock; when it requires a password, screenIsLocked fires.
        let center = DistributedNotificationCenter.default()
        center.addObserver(forName: NSNotification.Name("com.apple.screenIsLocked"), object: nil, queue: nil) { [weak self] _ in
            self?.emitSessionLocked()
        }
        NSWorkspace.shared.notificationCenter.addObserver(forName: NSWorkspace.willSleepNotification, object: nil, queue: nil) { [weak self] _ in
            self?.emitSessionLocked()
        }

        let stdinHandle = FileHandle.standardInput
        stdinHandle.readabilityHandler = { [weak self] handle in
            let data = handle.availableData
            if data.isEmpty {
                handle.readabilityHandler = nil
                self?.cancelAllAuthorizations()
                exit(0)
            }
            self?.ingest(data)
        }
    }

    // Lid close fires both screenIsLocked and willSleep; one lock is enough.
    private func emitSessionLocked() {
        stateLock.lock()
        let now = Date()
        let duplicate = now.timeIntervalSince(lastLockEvent) < 1
        if !duplicate {
            lastLockEvent = now
        }
        stateLock.unlock()
        if !duplicate {
            emit(HelperResponse(id: nil, type: "event", status: "session_locked", provider: "local-authentication", message: nil))
        }
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
        if req.type == "cancel" {
            cancelRequest(id: req.id)
            return
        }
        guard let id = req.id, !id.isEmpty else {
            emit(HelperResponse(id: req.id, type: req.type, status: "failed", provider: "local-authentication", message: "missing request id"))
            return
        }
        guard claimRequest(id: id) else { return }

        switch req.type {
        case "authorize":
            authorize(request: req, id: id)
        case "collect-password":
            collectPassword(request: req, id: id)
        case "subscribe-locks":
            guard finishPendingRequest(id: id) else { return }
            emit(HelperResponse(id: id, type: req.type, status: "ok", provider: "local-authentication", message: nil))
        case "status":
            let (status, message) = capabilityStatus()
            guard finishPendingRequest(id: id) else { return }
            emit(HelperResponse(id: id, type: req.type, status: status, provider: "local-authentication", message: message))
        default:
            guard finishPendingRequest(id: id) else { return }
            emit(HelperResponse(id: id, type: req.type, status: "failed", provider: "local-authentication", message: "unsupported request"))
        }
    }

    private func authorize(request: HelperRequest, id: String) {
        let state = AuthorizationState()
        let context = state.context
        guard beginAuthorization(state, id: id) else { return }
        if #available(macOS 10.12.2, *) {
            context.touchIDAuthenticationAllowableReuseDuration = 0
        }
        var policyError: NSError?
        let policy: LAPolicy = .deviceOwnerAuthentication

        guard context.canEvaluatePolicy(policy, error: &policyError) else {
            guard finishAuthorization(id: id, state: state) else { return }
            let status = helperUnavailableStatus(policyError as? LAError)
            emit(HelperResponse(id: id, type: request.type, status: status, provider: "local-authentication", message: policyError?.localizedDescription))
            return
        }

        let reason = (request.reason?.isEmpty == false ? request.reason! : "Authenticate to continue")
        context.evaluatePolicy(policy, localizedReason: reason) { [weak self] success, error in
            guard let self else { return }
            guard self.finishAuthorization(id: id, state: state) else { return }
            if success {
                self.emit(HelperResponse(id: id, type: request.type, status: "ok", provider: "local-authentication", message: nil))
                return
            }

            if let laError = error as? LAError {
                switch laError.code {
                case .userCancel, .appCancel, .systemCancel:
                    self.emit(HelperResponse(id: id, type: request.type, status: "canceled", provider: "local-authentication", message: laError.localizedDescription))
                    return
                case .notInteractive:
                    self.emit(HelperResponse(id: id, type: request.type, status: "unavailable_by_environment", provider: "local-authentication", message: laError.localizedDescription))
                    return
                case .biometryNotAvailable, .biometryNotEnrolled, .biometryLockout, .passcodeNotSet:
                    self.emit(HelperResponse(id: id, type: request.type, status: "unavailable_by_platform", provider: "local-authentication", message: laError.localizedDescription))
                    return
                default:
                    break
                }
            }

            self.emit(HelperResponse(id: id, type: request.type, status: "failed", provider: "local-authentication", message: error?.localizedDescription))
        }
    }

    private func claimRequest(id: String) -> Bool {
        stateLock.lock()
        defer { stateLock.unlock() }
        guard activeRequests[id] == nil else { return false }
        activeRequests[id] = .pending
        return true
    }

    private func finishPendingRequest(id: String) -> Bool {
        stateLock.lock()
        defer { stateLock.unlock() }
        guard let request = activeRequests[id], case .pending = request else { return false }
        activeRequests.removeValue(forKey: id)
        return true
    }

    private func beginAuthorization(_ state: AuthorizationState, id: String) -> Bool {
        stateLock.lock()
        defer { stateLock.unlock() }
        guard let request = activeRequests[id], case .pending = request else { return false }
        activeRequests[id] = .authorization(state)
        return true
    }

    private func finishAuthorization(id: String, state: AuthorizationState) -> Bool {
        stateLock.lock()
        defer { stateLock.unlock() }
        guard let request = activeRequests[id], case .authorization(let current) = request, current === state else { return false }
        activeRequests.removeValue(forKey: id)
        return true
    }

    private func cancelRequest(id: String?) {
        guard let id, !id.isEmpty else { return }
        stateLock.lock()
        let request = activeRequests.removeValue(forKey: id)
        stateLock.unlock()
        switch request {
        case .authorization(let state):
            state.context.invalidate()
        case .password(let prompt):
            abortPasswordPrompt(prompt.alert)
        case .pending, nil:
            break
        }
    }

    private func cancelAllAuthorizations() {
        stateLock.lock()
        let requests = Array(activeRequests.values)
        activeRequests.removeAll()
        stateLock.unlock()
        for request in requests {
            switch request {
            case .authorization(let state):
                state.context.invalidate()
            case .password(let prompt):
                abortPasswordPrompt(prompt.alert)
            case .pending:
                break
            }
        }
    }

    // collectPassword shows a native master-password prompt. Used for external
    // (SSH/signing) use once the device-unlock window has lapsed, where there is
    // no TUI to fall back to. Must run on the main thread for AppKit.
    private func collectPassword(request: HelperRequest, id: String) {
        let prompt = PasswordPromptState()
        guard beginPasswordPrompt(prompt, id: id) else { return }

        DispatchQueue.main.async { [weak self] in
            guard let self else { return }

            let alert = NSAlert()
            alert.messageText = "Forged is locked"
            alert.informativeText = (request.reason?.isEmpty == false ? request.reason! : "Enter your Forged master password to continue.")
            alert.addButton(withTitle: "Unlock")
            alert.addButton(withTitle: "Cancel")

            let field = NSSecureTextField(frame: NSRect(x: 0, y: 0, width: 260, height: 24))
            field.placeholderString = "Master password"
            alert.accessoryView = field
            alert.window.initialFirstResponder = field

            guard self.attachPasswordAlert(alert, prompt: prompt, id: id) else { return }
            guard self.activatePasswordPrompt(prompt, id: id) else { return }

            let result = alert.runModal()
            guard self.finishPasswordPrompt(prompt, id: id) else { return }
            if result == .alertFirstButtonReturn {
                let encoded = Data(field.stringValue.utf8).base64EncodedString()
                self.emit(HelperResponse(id: id, type: request.type, status: "ok", provider: "local-authentication", message: nil, secret: encoded))
            } else {
                self.emit(HelperResponse(id: id, type: request.type, status: "canceled", provider: "local-authentication", message: nil))
            }
        }
    }

    private func beginPasswordPrompt(_ prompt: PasswordPromptState, id: String) -> Bool {
        stateLock.lock()
        defer { stateLock.unlock() }
        guard let request = activeRequests[id], case .pending = request else { return false }
        activeRequests[id] = .password(prompt)
        return true
    }

    private func attachPasswordAlert(_ alert: NSAlert, prompt: PasswordPromptState, id: String) -> Bool {
        stateLock.lock()
        defer { stateLock.unlock() }
        guard let request = activeRequests[id], case .password(let current) = request, current === prompt else { return false }
        prompt.alert = alert
        return true
    }

    private func activatePasswordPrompt(_ prompt: PasswordPromptState, id: String) -> Bool {
        stateLock.lock()
        defer { stateLock.unlock() }
        guard let request = activeRequests[id], case .password(let current) = request, current === prompt else { return false }
        NSApp.activate(ignoringOtherApps: true)
        return true
    }

    private func finishPasswordPrompt(_ prompt: PasswordPromptState, id: String) -> Bool {
        stateLock.lock()
        defer { stateLock.unlock() }
        guard let request = activeRequests[id], case .password(let current) = request, current === prompt else { return false }
        activeRequests.removeValue(forKey: id)
        return true
    }

    private func abortPasswordPrompt(_ alert: NSAlert?) {
        guard let alert else { return }
        RunLoop.main.perform(inModes: [.default, .modalPanel, .eventTracking]) {
            if NSApp.modalWindow === alert.window {
                NSApp.abortModal()
            }
            alert.window.orderOut(nil)
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
runtime.start()

// Accessory app: no Dock icon, but can show the master-password popup and take
// keyboard focus. app.run() drains the main GCD queue on the real main thread,
// which AppKit's NSAlert requires.
let app = NSApplication.shared
app.setActivationPolicy(.accessory)
app.run()

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
