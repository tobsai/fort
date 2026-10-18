export class OwnerRecoveryIdentityRejected extends Error {
  constructor(identity: 'installation' | 'device') {
    super(identity === 'installation' ? 'Recovery requires this exact installation.' : 'Recovery requires the latest exact owner pairing.');
  }
}
