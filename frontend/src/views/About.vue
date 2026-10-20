<!-- 关于 / 数据说明：静态中文说明页 -->
<script setup lang="ts">
// 纯静态页面，无逻辑
</script>

<template>
  <div class="card about">
    <h1 class="page-title">关于 / 数据说明</h1>
    <p class="page-desc">游戏榜单监控平台的数据口径、来源与已知局限。</p>

    <h2>🌍 双区策略</h2>
    <ul>
      <li><strong>App Store 同时看中国区（cn）与美国区（us）</strong>：中美是全球最大的两个手游市场，同一款游戏在两区的排名差异能反映其区域化表现。</li>
      <li><strong>Steam 为全球榜（global）</strong>：Steam 不分国区榜单（其"热门"基于全球同时在线人数），因此只维护全球榜。</li>
      <li><strong>Google Play 默认美国区（us）</strong>：Google Play 在中国大陆不可用，故不设中国区，默认展示美国区。</li>
    </ul>

    <h2>📡 各平台数据来源与更新频率</h2>
    <ul>
      <li><strong>Steam</strong>：通过 Steam 公开接口采集热门游戏在线人数与排名，每日榜每日更新、每周榜每周汇总。在线人数为平台公开数据，可信度高。</li>
      <li><strong>App Store</strong>：通过 iTunes RSS 榜单采集中国区 / 美国区的热门免费榜与新游榜（游戏分类过滤），每日更新。</li>
      <li><strong>Google Play</strong>：Google 未提供公开榜单 API。旧版官方榜单页已下线，当前通过 AppBrain 的 Google Play 榜单镜像页（美国区游戏榜，每日从 Google Play 更新）抓取热门免费榜 / 新游免费榜。抓取遵守礼貌策略：请求间隔 ≥2 秒、单线程、命中 429/403/验证码即停。抓取字段：排名、包名、游戏名、开发者、图标、评分、累计下载。</li>
      <li><strong>玩家讨论</strong>：聚合 Steam 评测、应用商店评论等公开渠道的玩家留言，仅作舆情参考，不代表平台立场。</li>
    </ul>

    <h2>⚠️ 数据渠道风险说明</h2>
    <ul>
      <li><strong>Google Play（第三方抓取，非官方）</strong>：抓取 AppBrain 镜像页违反 Google Play 服务条款精神、且镜像页本身可能被反爬；对方页面改版即失效，需要持续维护解析器。前端与接口已如实标注"第三方渠道抓取，非官方"。URL 可通过环境变量 <code>GOOGLEPLAY_CHART_URL_HOT</code> / <code>GOOGLEPLAY_CHART_URL_NEW</code> 覆盖。</li>
    </ul>

    <h2>🏷️ 标签分类策略（IAA / 单机 / 个人游戏）</h2>
    <p>游戏标签用于快速识别一款游戏的题材与变现模式，采用「平台标签 + 变现模式标签」双维度：</p>
    <ul>
      <li><strong>平台标签</strong>：直接取自各平台官方分类（如"射击""MOBA""三消""RPG"），保持与平台口径一致。</li>
      <li><strong>变现模式标签</strong>：
        <ul>
          <li><span class="tag">IAA</span> 以广告变现为主的超休闲 / 休闲游戏（免费 + 激励视频广告）；</li>
          <li><span class="tag">IAP</span> 以内购变现为主的游戏（免费 + 道具 / 抽卡付费）；</li>
          <li><span class="tag">单机</span> 买断制、一次性付费、无内购的游戏；</li>
          <li><span class="tag">个人游戏</span> 独立开发者或小团队作品，常与"独立"标签并存。</li>
        </ul>
      </li>
      <li><strong>标注流程</strong>：规则初筛（按价格、内购标识、品类关键词自动打标）+ 人工标注复核。规则无法判定的进入人工队列，由运营同学确认后再上榜展示。</li>
    </ul>

    <h2>⚠️ 已知局限</h2>
    <ul>
      <li>Google Play 榜单来自第三方镜像抓取（非官方），可能有延迟，且对方改版会导致采集失效。</li>
      <li>非 Steam 平台不提供公开在线人数，详情页会提示"该平台无公开人数数据"。</li>
      <li>新游榜的"新"以各平台榜单口径为准，不同平台对新游的定义可能不一致。</li>
      <li>上榜理由（首次上榜 / 大幅上升等）基于相邻周期排名变化计算得出，仅供参考，不构成投资或发行建议。</li>
      <li>玩家讨论来自公开评论的抽样聚合，可能存在抽样偏差与水军噪音。</li>
      <li>榜单数据每日采集一次，存在最多 1 天的延迟；手动采集可触发即时更新。</li>
    </ul>
  </div>
</template>
