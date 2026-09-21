declare module '*.css';
interface Window {
  go: { main: { App: {
    Namespaces(): Promise<string[]>;
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
