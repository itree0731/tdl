declare module '*.css';
interface Window {
  go: { main: { App: {
    Namespaces(): Promise<string[]>;
    SelectUploadFiles(): Promise<string[]>;
    SelectUploadDirectory(): Promise<string>;
    StartUpload(request: unknown): Promise<{accepted:boolean;message:string}>;
    StopTransfer(): Promise<boolean>;
  }}};
  runtime: { EventsOn(name:string, callback:(data:any)=>void):()=>void };
}
