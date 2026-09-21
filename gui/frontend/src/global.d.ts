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
    ChatPage(namespace:string, cursor:number, limit:number): Promise<{items:Array<{id:number;username:string;title:string;type:string;topics:Array<{id:number;title:string}>;self:boolean}>;next:number;skipped:number}>;
  }}};
  runtime: { EventsOn(name:string, callback:(data:any)=>void):()=>void };
}
