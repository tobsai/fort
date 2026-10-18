#if os(macOS)
import Foundation
import Darwin
import Testing
@testable import FortKit

@Test(.enabled(if:ProcessInfo.processInfo.environment["FORT_TEST_NODE"] != nil),arguments:["success","nonzero","changed-storage"])
func bundledHistoryMaintenanceRequiresCleanExitAndTheSameStorage(_ scenario: String) async throws {
    let fixture = try HostBundleFixture(); defer { try? fixture.remove() }
    let bundle = try MacHostBundle.inspect(applicationURL:fixture.app,signature:{ _,_,_ in fixture.signature })
    let storage = try MacAgentStorage.prepare(for:fixture.endpoint,resolveContainer:{ _ in fixture.root })
    let directory = try storage.validatedDirectory()
    let resources = fixture.helper.appendingPathComponent("Contents/Resources/host")
    try FileManager.default.createDirectory(at:resources,withIntermediateDirectories:true,attributes:[.posixPermissions:0o700])
    try Data("nodejs_conf = fort_node\n[fort_node]\n".utf8).write(to:resources.appendingPathComponent("node-openssl.cnf"))
    let script = #"""
    import { writeFileSync, renameSync, mkdirSync } from 'node:fs';
    const directory = process.cwd();
    writeFileSync(directory + '/worker-pid',String(process.pid));
    process.stdout.write(JSON.stringify({protocolVersion:1,kind:'ready'})+'\n');
    process.stdin.once('data',data=>{
      const command=JSON.parse(data.toString()),request=command.history;
      const body={installationId:request.installationId,deviceId:request.deviceId,deletionId:request.deletionId,previousRecoveryEpoch:request.recoveryEpoch,recoveryEpoch:'next',deletedRecords:4};
      process.stdout.write(JSON.stringify({protocolVersion:1,kind:'response',id:command.id,response:{status:200,body:Buffer.from(JSON.stringify(body)).toString('base64')}})+'\n');
    });
    process.stdin.once('end',()=>{
      if ('SCENARIO' === 'changed-storage') {renameSync(directory,directory+'.moved');mkdirSync(directory,{mode:0o700});}
      if ('SCENARIO' === 'nonzero') process.exitCode=1;
    });
    """#.replacingOccurrences(of:"SCENARIO",with:scenario)
    try Data(script.utf8).write(to:resources.appendingPathComponent("history-maintenance.mjs"))
    let bootstrap = resources.appendingPathComponent("main.mjs")
    try Data("throw new Error('Ordinary host must not start');".utf8).write(to:bootstrap)
    let environment = ProcessInfo.processInfo.environment
    let program = MacHostProgram(bundle:bundle,executable:URL(fileURLWithPath:try #require(environment["FORT_TEST_NODE"])),guardian:URL(fileURLWithPath:try #require(environment["FORT_TEST_GUARDIAN"])),bootstrap:bootstrap,codex:fixture.root.appendingPathComponent("absent-codex"))
    let request = try MacHistoryDeletionRequest(installationID:"installation",deviceID:"device",recoveryEpoch:"old",deletionID:"confirmed")
    if scenario == "success" {
        #expect(try await program.deleteHistory(request,storage:storage).recoveryEpoch == "next")
    } else {
        await #expect(throws:(any Error).self) { try await program.deleteHistory(request,storage:storage) }
    }
    let location = scenario == "changed-storage" ? directory.appendingPathExtension("moved") : directory
    let pid = try #require(Int32(String(contentsOf:location.appendingPathComponent("worker-pid"),encoding:.utf8)))
    #expect(kill(pid,0) == -1 && errno == ESRCH)
    #expect(kill(-pid,0) == -1 && errno == ESRCH)
}
#endif
