package com.temproot.manager

import android.app.Activity
import android.net.LocalSocket
import android.net.LocalSocketAddress
import android.os.Bundle
import android.widget.Button
import android.widget.LinearLayout
import android.widget.TextView
import org.json.JSONObject
import java.io.BufferedReader
import java.io.InputStreamReader
import java.io.OutputStreamWriter

class MainActivity : Activity() {
    private lateinit var statusView: TextView

    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        val container = LinearLayout(this).apply {
            orientation = LinearLayout.VERTICAL
            setPadding(32, 32, 32, 32)
        }
        statusView = TextView(this).apply {
            text = "TempRoot Manager V0.1\n服务状态尚未检查"
            textSize = 18f
        }
        container.addView(statusView)
        container.addView(Button(this).apply {
            text = "检查授权服务"
            setOnClickListener { checkService() }
        })
        setContentView(container)
    }

    private fun checkService() {
        statusView.text = "正在检查本地授权服务..."
        Thread {
            val result = try {
                LocalSocket().use { socket ->
                    socket.connect(
                        LocalSocketAddress(
                            "/data/local/tmp/temproot.sock",
                            LocalSocketAddress.Namespace.FILESYSTEM
                        )
                    )
                    val writer = OutputStreamWriter(socket.outputStream, Charsets.UTF_8)
                    writer.write(JSONObject().put("op", "status").toString() + "\n")
                    writer.flush()
                    BufferedReader(InputStreamReader(socket.inputStream)).readLine()
                        ?: "服务没有返回状态"
                }
            } catch (e: Exception) {
                "未连接到授权服务：${e.message}"
            }
            runOnUiThread { statusView.text = result }
        }.start()
    }
}
