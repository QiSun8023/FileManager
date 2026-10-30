package io.github.qisun8023.filemanager.helper;
import android.util.Base64;
import org.json.JSONObject;
import java.io.*;
import java.nio.charset.StandardCharsets;
import java.util.concurrent.TimeUnit;
final class Root {
 static final String BIN="/data/adb/modules/filemanager/bin/filemanager";
 static JSONObject call(String action,JSONObject data)throws Exception{
  if(!action.equals("watch-status")&&!action.equals("watch-config")&&!action.equals("watch-save")&&!action.equals("watch-enable")&&!action.equals("directories")&&!action.equals("watch-start")&&!action.equals("watch-decide"))throw new IOException("Unsupported action");
  String encoded=Base64.encodeToString(data.toString().getBytes(StandardCharsets.UTF_8),Base64.NO_WRAP);
  Process process=new ProcessBuilder("su","-c",BIN+" "+action+" '"+encoded+"'").redirectErrorStream(true).start();
  ByteArrayOutputStream out=new ByteArrayOutputStream();Thread reader=new Thread(()->{try(InputStream in=process.getInputStream()){byte[] b=new byte[4096];int n;while((n=in.read(b))!=-1){if(out.size()+n>8388608){process.destroy();break;}out.write(b,0,n);}}catch(IOException ignored){}});reader.start();
  long timeout=(action.equals("watch-status")||action.equals("watch-config")||action.equals("watch-start"))?20:600;
  try{if(!process.waitFor(timeout,TimeUnit.SECONDS)){process.destroyForcibly();throw new IOException("Root 连接超时，将自动重试；请检查后台限制及 Root 授权");}reader.join(2000);}catch(InterruptedException ex){process.destroyForcibly();Thread.currentThread().interrupt();throw new IOException("Root 操作已取消",ex);}finally{if(process.isAlive())process.destroyForcibly();}
  String raw=out.toString("UTF-8").trim();JSONObject response;try{response=new JSONObject(raw);}catch(Exception ex){throw new IOException("未连接模块，请确认已安装新版模块，并在 KernelSU 中允许文件管理助手使用 Root");}
  if(!response.optBoolean("ok"))throw new IOException(response.optString("error","操作失败"));return response.optJSONObject("data");
 }
}
