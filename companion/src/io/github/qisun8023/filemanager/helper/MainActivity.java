package io.github.qisun8023.filemanager.helper;
import android.app.*;import android.os.*;import android.content.*;import android.net.Uri;import android.provider.Settings;import android.widget.*;import android.view.*;import org.json.*;
public class MainActivity extends Activity {
 TextView status;boolean connected=false;
 public void onCreate(Bundle b){super.onCreate(b);LinearLayout content=new LinearLayout(this);content.setOrientation(1);int pad=(int)(24*getResources().getDisplayMetrics().density);content.setPadding(pad,pad,pad,pad);ScrollView scroll=new ScrollView(this);scroll.addView(content);setContentView(scroll);
  TextView title=new TextView(this);title.setText("文件管理助手");title.setTextSize(26);content.addView(title);
  TextView intro=new TextView(this);intro.setText("为模块提供新文件通知与悬浮询问。监控目录、类型和目标目录请在模块 WebUI 中设置。\n\n首次使用依次允许 Root、通知和悬浮窗；关闭此页面后仍可提醒。文件只有在你选择转移后才会移动。\n");content.addView(intro);
  status=new TextView(this);content.addView(status);
  button(content,"1. 连接模块并允许 Root",()->connect());
  button(content,"2. 允许通知",()->{if(Build.VERSION.SDK_INT>=33)requestPermissions(new String[]{"android.permission.POST_NOTIFICATIONS"},7);else startActivity(new Intent(Settings.ACTION_APP_NOTIFICATION_SETTINGS).putExtra(Settings.EXTRA_APP_PACKAGE,getPackageName()));});
  button(content,"3. 允许显示在其他应用上层",()->startActivity(new Intent(Settings.ACTION_MANAGE_OVERLAY_PERMISSION,Uri.parse("package:"+getPackageName()))));
  button(content,"启动／恢复提醒",()->{if(!connected){connect();return;}getSharedPreferences("helper",0).edit().putBoolean("reminders",true).apply();try{startForegroundService(new Intent(this,MonitorService.class));status.setText("已启动提醒；请在 WebUI 启用监控规则。");}catch(Exception ex){status.setText(ex.toString());}});
  button(content,"暂停助手提醒",()->{getSharedPreferences("helper",0).edit().putBoolean("reminders",false).apply();stopService(new Intent(this,MonitorService.class));status.setText("助手提醒已暂停，文件留在原处，WebUI 待处理记录仍保留。");});
  TextView note=new TextView(this);note.setText("\n请在 ColorOS 中允许本应用后台运行／自启动（如系统要求）。悬浮窗仅在屏幕已解锁时显示；锁屏期间使用通知。通知被关闭时请在 WebUI 查看待处理文件。\n\n首次启用监控会忽略已有文件；关闭期间的文件在再次启用时也作为已有文件。ZIP／APK 会检查压缩包完整性，其他格式使用稳定等待判断，仍需核对下载状态。");content.addView(note);
 }
 void button(LinearLayout box,String label,Runnable fn){Button b=new Button(this);b.setText(label);box.addView(b,new LinearLayout.LayoutParams(-1,-2));b.setOnClickListener(v->fn.run());}
 void connect(){status.setText("正在连接，请留意 KernelSU 授权询问…");new Thread(()->{try{JSONObject data=Root.call("watch-status",new JSONObject());connected=true;runOnUiThread(()->status.setText("已连接模块 "+data.optString("version")+"。请继续开启通知与悬浮窗权限。"));}catch(Exception ex){runOnUiThread(()->status.setText(ex.getMessage()));}}).start();}
}
