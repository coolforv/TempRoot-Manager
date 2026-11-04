package com.temproot.manager

import android.app.Activity
import android.app.AlertDialog
import android.net.LocalSocket
import android.net.LocalSocketAddress
import android.os.Bundle
import android.view.ViewGroup
import android.widget.Button
import android.widget.LinearLayout
import android.widget.ScrollView
import android.widget.TextView
import org.json.JSONArray
import org.json.JSONObject
import java.io.BufferedReader
import java.io.InputStreamReader
import java.io.OutputStreamWriter

class MainActivity : Activity() {
    private lateinit var status: TextView
    private lateinit var pendingSection: LinearLayout
    private lateinit var grantsSection: LinearLayout
    private val socketPath = "/data/local/tmp/temproot.sock"

    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        title = "TempRoot Manager V0.2"
        val scroll = ScrollView(this)
        val main = LinearLayout(this).apply {
            orientation = LinearLayout.VERTICAL
            setPadding(32, 24, 32, 32)
        }
        scroll.addView(main)
        main.addView(label("TempRoot Manager V0.2", 23f))
        main.addView(label("不使用 /data/adb。仅管理已由受信任 Root 进程启动的本地服务。", 13f))
        status = label("尚未检查授权服务", 15f)
        main.addView(status)
        main.addView(button("刷新服务、申请与授权列表") { refresh() })
        main.addView(label("待处理 Root 申请", 19f))
        pendingSection = LinearLayout(this).apply { orientation = LinearLayout.VERTICAL }
        main.addView(pendingSection)
        main.addView(label("已授权应用", 19f))
        grantsSection = LinearLayout(this).apply { orientation = LinearLayout.VERTICAL }
        main.addView(grantsSection)
        main.addView(label("安全提示：拒绝默认授权。只有已确认来源的应用才能获得权限。Root 会话结束后应终止 Broker。", 12f))
        setContentView(scroll)
        refresh()
    }

    private fun label(value: String, size: Float) = TextView(this).apply {
        text = value
        textSize = size
        setPadding(0, 12, 0, 12)
        layoutParams = LinearLayout.LayoutParams(ViewGroup.LayoutParams.MATCH_PARENT, ViewGroup.LayoutParams.WRAP_CONTENT)
    }
    private fun button(value: String, action: () -> Unit) = Button(this).apply {
        text = value
        setOnClickListener { action() }
    }
    private fun packagesFor(uid: Int): String {
        val names = packageManager.getPackagesForUid(uid)
        return if (names.isNullOrEmpty()) "UID $uid（包名不可见）" else names.joinToString(", ") + " (UID $uid)"
    }
    private fun call(request: JSONObject): JSONObject {
        LocalSocket().use { socket ->
            socket.connect(LocalSocketAddress(socketPath, LocalSocketAddress.Namespace.FILESYSTEM))
            socket.soTimeout = 3500
            val output = OutputStreamWriter(socket.outputStream, Charsets.UTF_8)
            output.write(request.toString() + "\n")
            output.flush()
            val line = BufferedReader(InputStreamReader(socket.inputStream, Charsets.UTF_8)).readLine()
                ?: throw IllegalStateException("服务未返回数据")
            return JSONObject(line)
        }
    }
    private fun refresh() {
        status.text = "正在读取服务状态..."
        Thread {
            val result = runCatching { call(JSONObject().put("op", "list")) }
            runOnUiThread {
                result.onSuccess {
                    if (!it.optBoolean("ok", false)) {
                        showUnavailable(it.optString("error", "服务拒绝请求"))
                    } else {
                        status.text = "授权服务已连接 · EUID=${it.optInt("euid", -1)}"
                        renderEntries(pendingSection, it.optJSONArray("pending"), true)
                        renderEntries(grantsSection, it.optJSONArray("grants"), false)
                    }
                }.onFailure {
                    showUnavailable("服务未连接：${it.javaClass.simpleName} ${it.message ?: ""}")
                }
            }
        }.start()
    }
    private fun showUnavailable(message: String) {
        status.text = message + "\n需要先由 GhostLock 的受信任 Root worker 启动 Broker。安装本 APK 不会自动获得 Root。"
        pendingSection.removeAllViews()
        grantsSection.removeAllViews()
    }
    private fun renderEntries(section: LinearLayout, values: JSONArray?, pending: Boolean) {
        section.removeAllViews()
        if (values == null || values.length() == 0) {
            section.addView(label(if (pending) "没有待处理申请" else "尚无授权应用", 13f))
            return
        }
        for (i in 0 until values.length()) {
            val uid = values.optInt(i, -1)
            if (uid < 10000) continue
            section.addView(label(packagesFor(uid), 14f))
            if (pending) {
                section.addView(button("允许此 UID 的 Root 申请") { confirmAction(uid, "grant") })
                section.addView(button("拒绝申请") { perform(uid, "deny") })
            } else {
                section.addView(button("撤销授权") { confirmAction(uid, "revoke") })
            }
        }
    }
    private fun confirmAction(uid: Int, action: String) {
        val text = if (action == "grant") "允许 Root" else "撤销 Root"
        AlertDialog.Builder(this)
            .setTitle(text)
            .setMessage("目标：${packagesFor(uid)}\nRoot 权限可能影响全部设备数据。请确认应用身份和用途。")
            .setNegativeButton("取消", null)
            .setPositiveButton("确认") { _, _ -> perform(uid, action) }
            .show()
    }
    private fun perform(uid: Int, action: String) {
        status.text = "正在处理 UID $uid..."
        Thread {
            val result = runCatching {
                call(JSONObject().put("op", action).put("uid", uid))
            }
            runOnUiThread {
                result.onFailure { status.text = "处理失败：${it.message}" }
                result.onSuccess { status.text = if (it.optBoolean("ok")) "操作完成" else "操作失败：${it.optString("error")}" }
                refresh()
            }
        }.start()
    }
}
