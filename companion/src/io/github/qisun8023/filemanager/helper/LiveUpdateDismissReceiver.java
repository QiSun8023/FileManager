package io.github.qisun8023.filemanager.helper;

import android.app.NotificationManager;
import android.content.*;
import java.util.*;

public class LiveUpdateDismissReceiver extends BroadcastReceiver {
    @Override public void onReceive(Context context, Intent intent) {
        ArrayList<String> ids = intent.getStringArrayListExtra("ids");
        if (ids == null || ids.size() > 500) return;
        synchronized (LiveUpdates.LOCK) {
            SharedPreferences preferences = context.getSharedPreferences("helper", 0);
            Set<String> hidden = new HashSet<>(preferences.getStringSet(LiveUpdates.HIDDEN, Collections.emptySet()));
            for (String id : ids) if (id != null && id.matches("[a-f0-9]{32}")) hidden.add(id);
            preferences.edit().putStringSet(LiveUpdates.HIDDEN, hidden).apply();
            context.getSystemService(NotificationManager.class).cancel(LiveUpdates.ID);
        }
    }
}
