export type OwnerSetupReceipt = Readonly<{
  installationId: string;
  deviceId: string;
  expiresAt: string | null;
}>;

export type OwnerSetupState =
  | Readonly<{ kind: 'available'; installationId: string; setupId: string; expiresAt: string }>
  | (Readonly<{ kind: 'paired' }> & OwnerSetupReceipt);

export type OwnerSetupIdentity = Readonly<{
  installationId: string;
  setupId: string;
  deviceId: string;
}>;

export type OwnerSetupRequest = OwnerSetupIdentity & Readonly<{ deviceToken: string }>;

export type OwnerSetupCancellation = OwnerSetupIdentity & (
  | Readonly<{ outcome: 'notPaired' }>
  | Readonly<{ outcome: 'revoked'; runtimeStopped: true }>
);
