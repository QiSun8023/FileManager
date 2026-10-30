package io.github.qisun8023.filemanager.helper;

import android.app.*;
import android.content.*;
import android.os.*;
import org.json.*;
import java.util.*;

/** A single promoted card for the pending queue; ordinary item notifications remain available. */
final class LiveUpdates {
    static final int ID = 3;
    static final String CHANNEL = "live-files";
    static final Object LOCK = new Object();
    static final String HIDDEN = "live-hidden-items";
    private String lastContent = "";

    static boolean supported() { return Build.VERSION.SDK_INT >= 36; }
    static boolean allowed(Context context) {
        return supported() && context.getSystemService(NotificationManager.class).canPostPromotedNotifications();
    }

    void update(MonitorService service, JSONArray items) throws JSONException {
        synchronized (LOCK) {
            NotificationManager manager = service.manager;
            SharedPreferences preferences = service.getSharedPreferences("helper", 0);
            Set<String> active = new HashSet<>();
            if (items != null) for (int i = 0; i < items.length(); i++) active.add(items.getJSONObject(i).getString("id"));
            Set<String> hidden = new HashSet<>(preferences.getStringSet(HIDDEN, Collections.emptySet()));
            hidden.retainAll(active);
            preferences.edit().putStringSet(HIDDEN, hidden).apply();
            if (service.stopped || active.isEmpty() || hidden.containsAll(active) || !service.canNotify() || !allowed(service)) {
                clear(manager);
                return;
            }
            JSONObject first = null;
            for (int i = 0; i < items.length(); i++) {
                JSONObject item = items.getJSONObject(i);
                if (!hidden.contains(item.getString("id"))) { first = item; break; }
            }
            if (first == null) { clear(manager); return; }
            String id = first.getString("id"), path = first.getString("path");
            String target = service.destination(first);
            String title = active.size() + " 个新文件待处理";
            String text = new java.io.File(path).getName();
            String details = text + "\n来源：" + path + "\n转移至：" + target
                + (first.optBoolean("verified") ? "\n已通过压缩包完整性检查" : "\n按稳定时间判断，请核对是否下载完成")
                + "\n按钮仅处理当前文件；点击查看全部待处理项。";
            String content = new TreeSet<>(active).toString() + id + details;
            // Never repost an unchanged card, including after a system/user demotion.
            if (content.equals(lastContent)) return;
            Intent dismiss = new Intent(service, LiveUpdateDismissReceiver.class)
                .putStringArrayListExtra("ids", new ArrayList<>(active));
            PendingIntent hide = PendingIntent.getBroadcast(service, 3, dismiss,
                PendingIntent.FLAG_IMMUTABLE | PendingIntent.FLAG_UPDATE_CURRENT);
            Bundle extras = new Bundle();
            // Official EXTRA_REQUEST_PROMOTED_ONGOING wire key (also used by AndroidX).
            // The named framework constant is only available in SDK 36.1+.
            extras.putBoolean("android.requestPromotedOngoing", true);
            Notification notification = new Notification.Builder(service, CHANNEL)
                .setSmallIcon(android.R.drawable.stat_sys_download_done)
                .setContentTitle(title).setContentText(text)
                .setStyle(new Notification.BigTextStyle().bigText(details))
                .setContentIntent(service.open()).setOngoing(true).setOnlyAlertOnce(true)
                .setShowWhen(false).setVisibility(Notification.VISIBILITY_PRIVATE)
                .setShortCriticalText(active.size() > 99 ? "99+待处理" : active.size() + "待处理")
                .addExtras(extras).setDeleteIntent(hide)
                .addAction(new Notification.Action.Builder(null, "转移", service.decision(id, "move")).build())
                .addAction(new Notification.Action.Builder(null, "保留原处", service.decision(id, "keep")).build())
                .addAction(new Notification.Action.Builder(null, "收起", hide).build()).build();
            manager.notify(ID, notification);
            lastContent = content;
        }
    }

    void clear(NotificationManager manager) {
        synchronized (LOCK) { manager.cancel(ID); lastContent = ""; }
    }
}
