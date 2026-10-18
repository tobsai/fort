export type OwnerSetupReceipt = Readonly<{
  installationId: string;
  deviceId: string;
  expiresAt: string | null;
}>;

export type OwnerSetupState =
  | Readonly<{ kind: 'available'; installationId: string; setupId: string; expiresAt: string }>
  | (Readonly<{ kind: 'paired' }> & OwnerSetupReceipt);

export type OwnerSetupRequest = Readonly<{
  installationId: string;
  setupId: string;
  deviceId: string;
  deviceToken: string;
}>;
