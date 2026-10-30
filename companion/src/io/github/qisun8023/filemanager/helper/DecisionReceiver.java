package io.github.qisun8023.filemanager.helper;
import android.app.*;import android.content.*;import android.widget.Toast;import android.os.Handler;import android.os.Looper;import org.json.JSONObject;
public class DecisionReceiver extends BroadcastReceiver {
 public void onReceive(Context c,Intent i){String id=i.getStringExtra("id"),decision=i.getStringExtra("decision");if(id==null||!id.matches("[a-f0-9]{32}")||!("move".equals(decision)||"keep".equals(decision)))return;PendingResult result=goAsync();new Thread(()->{try{JSONObject data=new JSONObject().put("id",id).put("decision",decision);Root.call("watch-decide",data);c.getSystemService(NotificationManager.class).cancel(id,2);new Handler(Looper.getMainLooper()).post(()->Toast.makeText(c,"move".equals(decision)?"已转移":"已保留原处",Toast.LENGTH_LONG).show());}catch(Exception ex){new Handler(Looper.getMainLooper()).post(()->Toast.makeText(c,ex.getMessage(),Toast.LENGTH_LONG).show());}finally{result.finish();}}).start();}
}
