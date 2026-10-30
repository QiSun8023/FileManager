package io.github.qisun8023.filemanager.helper;

import android.app.Dialog;
import android.content.Context;
import android.content.res.Configuration;
import android.graphics.Color;
import android.graphics.Typeface;
import android.graphics.drawable.ColorDrawable;
import android.graphics.drawable.GradientDrawable;
import android.text.TextUtils;
import android.util.TypedValue;
import android.view.Gravity;
import android.view.View;
import android.view.Window;
import android.view.WindowManager;
import android.widget.Button;
import android.widget.LinearLayout;
import android.widget.TextView;

/**
 * Overlay transfer prompt drawn with the Miuix dialog style: bottom anchored, 32dp corners,
 * 24dp inside margin, centered title and 16dp rounded buttons. Follows the system dark mode.
 */
final class MiuixPrompt {
  interface Choice { void pick(String decision); }

  private MiuixPrompt() {}

  static Dialog create(Context context, String fileName, String source, String target, boolean verified, Choice choice) {
    boolean dark = (context.getResources().getConfiguration().uiMode & Configuration.UI_MODE_NIGHT_MASK) == Configuration.UI_MODE_NIGHT_YES;
    int container = dark ? 0xFF242424 : 0xFFFFFFFF;
    int onSurface = dark ? 0xFFF2F2F2 : 0xFF000000;
    int summary = dark ? 0x80FFFFFF : 0x99000000;
    int field = dark ? 0xFF434343 : 0xFFF0F0F0;
    int onField = dark ? 0xFFD9D9D9 : 0xFF303030;
    int primary = dark ? 0xFF277AF7 : 0xFF3482FF;
    int ok = dark ? 0xFF2FC979 : 0xFF18A058;
    int warn = dark ? 0xFFFFA733 : 0xFFE07B00;

    Dialog dialog = new Dialog(context);
    dialog.requestWindowFeature(Window.FEATURE_NO_TITLE);

    LinearLayout root = new LinearLayout(context);
    root.setOrientation(LinearLayout.VERTICAL);
    root.setPadding(dp(context, 24), dp(context, 24), dp(context, 24), dp(context, 24));
    root.setBackground(round(container, dp(context, 32)));

    TextView title = text(context, "是否转移新增文件？", 20, onSurface, true);
    title.setGravity(Gravity.CENTER);
    root.addView(title, block());

    TextView name = text(context, fileName, 16, onSurface, false);
    name.setGravity(Gravity.CENTER);
    name.setMaxLines(2);
    name.setEllipsize(TextUtils.TruncateAt.MIDDLE);
    LinearLayout.LayoutParams nameParams = block();
    nameParams.topMargin = dp(context, 12);
    root.addView(name, nameParams);

    TextView state = text(context, verified ? "完整性检查通过" : "仅通过稳定等待，请核对下载是否完成", 13, verified ? ok : warn, false);
    state.setGravity(Gravity.CENTER);
    LinearLayout.LayoutParams stateParams = block();
    stateParams.topMargin = dp(context, 4);
    root.addView(state, stateParams);

    LinearLayout route = new LinearLayout(context);
    route.setOrientation(LinearLayout.VERTICAL);
    route.setPadding(dp(context, 14), dp(context, 12), dp(context, 14), dp(context, 12));
    route.setBackground(round(field, dp(context, 12)));
    route.addView(text(context, "来源", 12, summary, false));
    route.addView(text(context, source, 14, onField, false));
    TextView toLabel = text(context, "转移至", 12, summary, false);
    toLabel.setPadding(0, dp(context, 8), 0, 0);
    route.addView(toLabel);
    route.addView(text(context, target, 14, onSurface, false));
    LinearLayout.LayoutParams routeParams = block();
    routeParams.topMargin = dp(context, 16);
    root.addView(route, routeParams);

    LinearLayout actions = new LinearLayout(context);
    actions.setOrientation(LinearLayout.HORIZONTAL);
    Button keep = button(context, "保留原处", field, onField);
    Button move = button(context, "转移", primary, Color.WHITE);
    LinearLayout.LayoutParams keepParams = new LinearLayout.LayoutParams(0, dp(context, 48), 1f);
    LinearLayout.LayoutParams moveParams = new LinearLayout.LayoutParams(0, dp(context, 48), 1f);
    moveParams.leftMargin = dp(context, 12);
    actions.addView(keep, keepParams);
    actions.addView(move, moveParams);
    LinearLayout.LayoutParams actionParams = block();
    actionParams.topMargin = dp(context, 24);
    root.addView(actions, actionParams);

    Button later = button(context, "稍后", Color.TRANSPARENT, primary);
    LinearLayout.LayoutParams laterParams = new LinearLayout.LayoutParams(LinearLayout.LayoutParams.MATCH_PARENT, dp(context, 44));
    laterParams.topMargin = dp(context, 4);
    root.addView(later, laterParams);

    keep.setOnClickListener(v -> { dialog.dismiss(); choice.pick("keep"); });
    move.setOnClickListener(v -> { dialog.dismiss(); choice.pick("move"); });
    later.setOnClickListener(v -> dialog.dismiss());

    dialog.setContentView(root);
    Window window = dialog.getWindow();
    window.setType(WindowManager.LayoutParams.TYPE_APPLICATION_OVERLAY);
    window.setBackgroundDrawable(new ColorDrawable(Color.TRANSPARENT));
    window.setGravity(Gravity.BOTTOM | Gravity.CENTER_HORIZONTAL);
    window.addFlags(WindowManager.LayoutParams.FLAG_DIM_BEHIND);
    window.setDimAmount(dark ? 0.55f : 0.3f);
    WindowManager.LayoutParams params = window.getAttributes();
    int screen = context.getResources().getDisplayMetrics().widthPixels;
    params.width = Math.min(screen - dp(context, 24), dp(context, 420));
    params.y = dp(context, 12);
    window.setAttributes(params);
    return dialog;
  }

  private static LinearLayout.LayoutParams block() {
    return new LinearLayout.LayoutParams(LinearLayout.LayoutParams.MATCH_PARENT, LinearLayout.LayoutParams.WRAP_CONTENT);
  }

  private static TextView text(Context context, String value, int sp, int color, boolean bold) {
    TextView view = new TextView(context);
    view.setText(value);
    view.setTextColor(color);
    view.setTextSize(TypedValue.COMPLEX_UNIT_SP, sp);
    view.setLineSpacing(0, 1.15f);
    if (bold) view.setTypeface(Typeface.create(Typeface.DEFAULT, Typeface.BOLD));
    return view;
  }

  private static Button button(Context context, String label, int background, int foreground) {
    Button view = new Button(context);
    view.setText(label);
    view.setAllCaps(false);
    view.setTextColor(foreground);
    view.setTextSize(TypedValue.COMPLEX_UNIT_SP, 16);
    view.setStateListAnimator(null);
    view.setElevation(0);
    view.setPadding(dp(context, 12), 0, dp(context, 12), 0);
    view.setBackground(round(background, dp(context, 16)));
    view.setTypeface(Typeface.create(Typeface.DEFAULT, Typeface.BOLD));
    return view;
  }

  private static GradientDrawable round(int color, int radius) {
    GradientDrawable shape = new GradientDrawable();
    shape.setColor(color);
    shape.setCornerRadius(radius);
    return shape;
  }

  private static int dp(Context context, int value) {
    return Math.round(value * context.getResources().getDisplayMetrics().density);
  }
}
