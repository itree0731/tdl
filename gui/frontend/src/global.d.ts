declare module '*.css';
interface Window {
  go: { main: { App: {
    Namespaces(): Promise<string[]>;
    GetSettings(): Promise<{schemaVersion:number;namespace:string;proxy:string;threads:number;limit:number;downloadDirectory:string;coverMode:string;coverAt:string;downloadGroup:boolean;downloadSkipSame:boolean;downloadRewrite:boolean}>;
    SaveSettings(settings: unknown): Promise<{schemaVersion:number;namespace:string;proxy:string;threads:number;limit:number;downloadDirectory:string;coverMode:string;coverAt:string;downloadGroup:boolean;downloadSkipSame:boolean;downloadRewrite:boolean}>;
    SelectUploadFiles(): Promise<string[]>;
    SelectUploadDirectory(): Promise<string>;
    SelectDownloadDirectory(): Promise<string>;
    SelectDownloadExportFiles(): Promise<string[]>;
    StartUpload(request: unknown): Promise<{accepted:boolean;message:string}>;
    StartDownload(request: unknown): Promise<{accepted:boolean;message:string}>;
    StopTransfer(): Promise<boolean>;
    MediaPreview(paths:string[]): Promise<{dataURL:string;kind:string;path:string;name:string;width:number;height:number}>;
    TaskHistory(): Promise<Array<{id:number;type:string;detail:string;status:string;summary:string;error:string;startedAt:string;finishedAt:string}>>;
    ClearTaskHistory(): Promise<void>;
    SelectForwardFiles(): Promise<string[]>;
    SelectChatExportDestination(): Promise<string>;
    SelectBackupDestination(): Promise<string>;
    SelectRecoveryFile(): Promise<string>;
    StartForward(request: unknown): Promise<{accepted:boolean;message:string}>;
    StartChatExport(request: unknown): Promise<{accepted:boolean;message:string}>;
    StartBackup(path:string): Promise<{accepted:boolean;message:string}>;
    StartRecover(path:string,confirmed:boolean): Promise<{accepted:boolean;message:string}>;
    ChatPage(namespace:string, cursor:number, limit:number): Promise<{items:Array<{id:number;username:string;title:string;type:string;topics:Array<{id:number;title:string}>;self:boolean}>;next:number;skipped:number}>;
  }}};
  runtime: { EventsOn(name:string, callback:(data:any)=>void):()=>void };
}
