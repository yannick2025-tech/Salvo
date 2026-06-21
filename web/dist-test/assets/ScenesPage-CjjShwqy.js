const __vite__mapDeps=(i,m=__vite__mapDeps,d=(m.f||(m.f=["assets/dashboard-112_7TEr.js","assets/client-rK0MEaiS.js","assets/chunk-62oNxeRG.js"])))=>i.map(i=>d[i]);
import{C as e,D as t,F as n,I as r,K as i,T as a,W as o,_t as s,bt as c,d as ee,f as te,i as l,k as ne,o as u,p as re,s as d,t as ie,u as ae}from"./runtime-core.esm-bundler-7VQ0jevk.js";import{i as f,s as p}from"./runtime-dom.esm-bundler-DXRKJZ33.js";import{i as oe}from"./vue-router-D1noiCHe.js";import{t as se}from"./preload-helper-Dd-HcVz_.js";import{t as m}from"./_plugin-vue_export-helper-DAAOZMkq.js";import{a as ce,n as le,r as ue,s as de}from"./scene-1A-js_5d.js";var fe={class:`scenes-page`},pe={class:`page-header`},me={class:`header-actions`},he={class:`table-wrapper`},ge={class:`data-table`},h={key:0},g={class:`mono`},_=[`title`],v={class:`time-cell`},y={class:`time-cell`},b={class:`time-cell`},x={class:`actions`},S=[`disabled`,`onClick`],C=[`onClick`],w={class:`modal`},T={class:`form-group`},E={class:`form-group`},D={class:`modal-actions`},O={class:`confirm-dialog`},k={class:`confirm-msg`},A={class:`confirm-actions`},j={class:`modal modal-lg`},M={class:`form-group`},N={class:`form-group`},P={key:0,class:`form-error`},F={class:`modal-actions`},I=[`disabled`],L=`name: Mock API 电商全链路压测
description: |
  基于 Mock Server 的完整电商链路压测场景，展示 Salvo 核心能力：
  - Setup/Teardown 生命周期管理
  - 参数生成器 (Generator) 动态生成请求参数
  - 参数关联 (Extract) 从响应提取变量传递给后续请求
  - IF-ELSE 分支 (IF-ELSE) 根据运行时条件走不同分支
  - 延迟控制 (Delay) 模拟用户思考时间

variables:
  - key: base_url
    value: http://localhost:9090/mock/api
  - key: token
    value: ""
  - key: user_id
    value: "0"
  - key: product_id
    value: "1"
  - key: order_id
    value: ""
  - key: payment_status
    value: ""

setup:
  - name: 注册测试用户
    type: setup
    config:
      method: POST
      url: "\${base_url}/users"
      headers:
        Content-Type: application/json
      body: '{"name":"perf_test_user","email":"perf@example.com"}'
      timeout: 5000
      expect_status: 201
      extract:
        user_id: "$.data.id"

  - name: 用户登录获取Token
    type: setup
    config:
      method: POST
      url: "\${base_url}/auth/login"
      headers:
        Content-Type: application/json
      body: '{"email":"admin@example.com","password":"admin123"}'
      timeout: 5000
      expect_status: 200
      extract:
        token: "$.data.token"
        user_id: "$.data.user.id"

nodes:
  - name: 浏览商品列表
    type: http
    config:
      method: GET
      url: "\${base_url}/products?page=1"
      headers:
        Authorization: "Bearer \${token}"
      timeout: 5000
      expect_status: 200
      extract:
        product_id: "$.data[0].id"

  - name: 查看商品详情
    type: http
    config:
      method: GET
      url: "\${base_url}/products/\${product_id}"
      headers:
        Authorization: "Bearer \${token}"
      timeout: 5000
      expect_status: 200

  - name: 用户思考时间
    type: delay
    config:
      ms: 500

  - name: 创建订单
    type: http
    config:
      method: POST
      url: "\${base_url}/orders"
      headers:
        Content-Type: application/json
        Authorization: "Bearer \${token}"
      body: '{"user_id":\${user_id},"total":79.98}'
      timeout: 5000
      expect_status: 201
      extract:
        order_id: "$.data.id"
      generator:
        body_fields:
          total:
            type: number
            minimum: 10
            maximum: 500
            multipleOf: 0.01

  - name: 判断是否支付
    type: if-else
    config:
      expr: "\${order_id} != ''"

  - name: 支付订单
    type: http
    config:
      method: POST
      url: "\${base_url}/payment"
      headers:
        Content-Type: application/json
        Authorization: "Bearer \${token}"
      body: '{"amount":79.98,"order_id":"\${order_id}"}'
      timeout: 5000
      expect_status: 200
      extract:
        payment_status: "$.data.status"
      generator:
        body_fields:
          amount:
            type: number
            minimum: 10
            maximum: 500

  - name: 跳过支付
    type: http
    config:
      method: GET
      url: "\${base_url}/orders/\${order_id}"
      headers:
        Authorization: "Bearer \${token}"
      timeout: 5000
      expect_status: 200

  - name: 发送支付通知
    type: http
    config:
      method: POST
      url: "\${base_url}/notify"
      headers:
        Content-Type: application/json
        Authorization: "Bearer \${token}"
      body: '{"event":"payment_success","order_id":"\${order_id}"}'
      timeout: 5000
      expect_status: 200

teardown:
  - name: 查询最终订单状态
    type: teardown
    config:
      method: GET
      url: "\${base_url}/orders/\${order_id}"
      headers:
        Authorization: "Bearer \${token}"
      timeout: 5000
      expect_status: 200

  - name: 清理测试数据
    type: teardown
    config:
      method: DELETE
      url: "\${base_url}/users/\${user_id}"
      headers:
        Authorization: "Bearer \${token}"
      timeout: 5000
      expect_status: 200

edges:
  - from: 注册测试用户
    to: 用户登录获取Token

  - from: 用户登录获取Token
    to: 浏览商品列表

  - from: 浏览商品列表
    to: 查看商品详情

  - from: 查看商品详情
    to: 用户思考时间

  - from: 用户思考时间
    to: 创建订单

  - from: 创建订单
    to: 判断是否支付

  - from: 判断是否支付
    to: 支付订单
    condition: "__if_true__"

  - from: 判断是否支付
    to: 跳过支付
    condition: "__if_false__"

  - from: 支付订单
    to: 发送支付通知

  - from: 发送支付通知
    to: 查询最终订单状态

  - from: 跳过支付
    to: 查询最终订单状态

  - from: 查询最终订单状态
    to: 清理测试数据
`,_e=`name: Mock API 电商全链路压测 V2（新版功能演示）
description: |
  基于 Mock Server 的完整电商链路压测场景，演示 Salvo 新版核心能力：
  - 多变量嵌套引用 (Nested Variables) - 三级引用链 A→B→C
  - CSV 数据源 (Data Source) - 用户凭据 + 商品数据双数据源参数化
  - 分组节点 (Group Node) - 订单处理子流程 ×3 循环 + 异步推荐 Group
  - 定时器节点 (Timer Node) - 延迟等待 + 间隔心跳检测
  - 节点级 LoopCount - HTTP 节点独立循环执行
  - Setup/Teardown 生命周期管理
  - IF-ELSE 条件分支
  - Generator 参数生成器

variables:
  - key: env
    value: staging
  - key: api_host
    value: \${env}-api.example.com
  - key: base_url
    value: http://\${api_host}/mock/api
  - key: token
    value: ""
  - key: user_id
    value: "0"
  - key: product_id
    value: "1"
  - key: order_id
    value: ""
  - key: payment_status
    value: ""
  - key: timeout_ms
    value: "5000"
  - key: retry_count
    value: "3"

data_sources:
  - name: user_credentials
    columns:
      - username
      - password
      - role
      - credit_limit
    rows:
      - username: "admin@example.com"
        password: "admin123"
        role: "admin"
        credit_limit: "5000"
      - username: "alice@example.com"
        password: "alice123"
        role: "user"
        credit_limit: "2000"
      - username: "bob@example.com"
        password: "bob123"
        role: "user"
        credit_limit: "1000"
      - username: "charlie@example.com"
        password: "charlie123"
        role: "user"
        credit_limit: "3000"

  # 商品数据源 — 演示 CSV 多列参数化（上传 products.csv 后自动关联）
  - name: products
    columns:
      - product_id
      - product_name
      - category
      - unit_price
    rows:
      - product_id: "1"
        product_name: "Widget A"
        category: "electronics"
        unit_price: "29.99"
      - product_id: "2"
        product_name: "Widget B"
        category: "electronics"
        unit_price: "49.99"
      - product_id: "3"
        product_name: "Gadget C"
        category: "gadgets"
        unit_price: "99.99"

setup:
  - name: 注册测试用户
    type: setup
    config:
      method: POST
      url: "\${base_url}/users"
      headers:
        Content-Type: application/json
      body: '{"name":"perf_user_\${user_credentials.username}","email":"\${user_credentials.username}","role":"\${user_credentials.role}"}'
      timeout: \${timeout_ms}
      expect_status: 201
      extract:
        user_id: "$.data.id"

  - name: 用户登录获取Token
    type: setup
    config:
      method: POST
      url: "\${base_url}/auth/login"
      headers:
        Content-Type: application/json
      body: '{"email":"\${user_credentials.username}","password":"\${user_credentials.password}"}'
      timeout: \${timeout_ms}
      expect_status: 200
      extract:
        token: "$.data.token"
        user_id: "$.data.user.id"

nodes:
  - name: 浏览商品列表
    type: http
    config:
      method: GET
      url: "\${base_url}/products?page=1"
      headers:
        Authorization: "Bearer \${token}"
      timeout: \${timeout_ms}
      expect_status: 200
      extract:
        product_id: "$.data[0].id"

  - name: 查看商品详情
    type: http
    config:
      method: GET
      url: "\${base_url}/products/\${product_id}"
      headers:
        Authorization: "Bearer \${token}"
      timeout: \${timeout_ms}
      expect_status: 200

  - name: 用户思考等待
    type: timer
    config:
      mode: delay
      delay: 500

  - name: 订单处理子步骤1：校验库存
    type: http
    config:
      method: GET
      url: "\${base_url}/inventory/\${product_id}"
      headers:
        Authorization: "Bearer \${token}"
      timeout: \${timeout_ms}
      expect_status: 200

  - name: 订单处理子步骤2：创建订单
    type: http
    config:
      method: POST
      url: "\${base_url}/orders"
      headers:
        Content-Type: application/json
        Authorization: "Bearer \${token}"
      body: '{"user_id":\${user_id},"product_id":\${product_id},"total":99.99}'
      timeout: \${timeout_ms}
      expect_status: 201
      extract:
        order_id: "$.data.id"
      generator:
        body_fields:
          total:
            type: number
            minimum: 10
            maximum: 500
            multipleOf: 0.01

  - name: 订单处理子步骤3：更新状态
    type: http
    config:
      method: PUT
      url: "\${base_url}/orders/\${order_id}/status"
      headers:
        Content-Type: application/json
        Authorization: "Bearer \${token}"
      body: '{"status":"confirmed"}'
      timeout: \${timeout_ms}
      expect_status: 200

  - name: 订单处理流程(Group)
    type: group
    config:
      node_ids:
        - 订单处理子步骤1：校验库存
        - 订单处理子步骤2：创建订单
        - 订单处理子步骤3：更新状态
      loop_count: 3

  - name: 判断是否支付
    type: if-else
    config:
      expr: "\${order_id} != ''"

  - name: 支付订单
    type: http
    config:
      method: POST
      url: "\${base_url}/payment"
      headers:
        Content-Type: application/json
        Authorization: "Bearer \${token}"
      body: '{"amount":99.99,"order_id":"\${order_id}"}'
      timeout: \${timeout_ms}
      expect_status: 200
      extract:
        payment_status: "$.data.status"
      generator:
        body_fields:
          amount:
            type: number
            minimum: 10
            maximum: 500

  - name: 跳过支付
    type: http
    config:
      method: GET
      url: "\${base_url}/orders/\${order_id}"
      headers:
        Authorization: "Bearer \${token}"
      timeout: \${timeout_ms}
      expect_status: 200

  - name: 支付结果轮询
    type: timer
    config:
      mode: interval
      interval: 1000
      duration: 5000

  - name: 发送支付通知
    type: http
    config:
      method: POST
      url: "\${base_url}/notify"
      headers:
        Content-Type: application/json
        Authorization: "Bearer \${token}"
      body: '{"event":"payment_success","order_id":"\${order_id}"}'
      timeout: \${timeout_ms}
      expect_status: 200

  # ---- CSV 多列参数化演示：使用 products 数据源 ----
  - name: 商品搜索(CSV参数化)
    type: http
    config:
      method: GET
      url: "\${base_url}/products/search?q=\${products.product_name}&category=\${products.category}"
      headers:
        Authorization: "Bearer \${token}"
      timeout: \${timeout_ms}
      expect_status: 200
    # 节点级 LoopCount：每个迭代使用 CSV 下一行
    loop_count: 3

  - name: 商品推荐(Async Group)
    type: group
    config:
      node_ids:
        - 异步商品详情查询
        - 异步价格计算
      loop_count: 2
      async: true

  - name: 异步商品详情查询
    type: http
    config:
      method: GET
      url: "\${base_url}/products/\${products.product_id}"
      headers:
        Authorization: "Bearer \${token}"
      timeout: \${timeout_ms}
      expect_status: 200

  - name: 异步价格计算
    type: http
    config:
      method: POST
      url: "\${base_url}/products/\${products.product_id}/price"
      headers:
        Content-Type: application/json
        Authorization: "Bearer \${token}"
      body: '{"unit_price":\${products.unit_price},"quantity":2}'
      timeout: \${timeout_ms}
      expect_status: 200

teardown:
  - name: 查询最终订单状态
    type: teardown
    config:
      method: GET
      url: "\${base_url}/orders/\${order_id}"
      headers:
        Authorization: "Bearer \${token}"
      timeout: \${timeout_ms}
      expect_status: 200

  - name: 清理测试数据
    type: teardown
    config:
      method: DELETE
      url: "\${base_url}/users/\${user_id}"
      headers:
        Authorization: "Bearer \${token}"
      timeout: \${timeout_ms}
      expect_status: 200

edges:
  - from: 注册测试用户
    to: 用户登录获取Token

  - from: 用户登录获取Token
    to: 浏览商品列表

  - from: 浏览商品列表
    to: 查看商品详情

  - from: 查看商品详情
    to: 用户思考等待

  - from: 用户思考等待
    to: 订单处理流程(Group)

  - from: 订单处理流程(Group)
    to: 判断是否支付

  - from: 判断是否支付
    to: 支付订单
    condition: "__if_true__"

  - from: 判断是否支付
    to: 跳过支付
    condition: "__if_false__"

  - from: 支付订单
    to: 支付结果轮询

  - from: 支付结果轮询
    to: 发送支付通知

  - from: 发送支付通知
    to: 商品搜索(CSV参数化)

  # CSV 参数化链路：商品搜索 → 异步推荐
  - from: 商品搜索(CSV参数化)
    to: 商品推荐(Async Group)

  - from: 商品推荐(Async Group)
    to: 查询最终订单状态

  - from: 跳过支付
    to: 查询最终订单状态

  - from: 查询最终订单状态
    to: 清理测试数据
`,R=m(re({__name:`ScenesPage`,setup(re){let m=oe(),R=i([]),z=i(!1),B=o({name:``,description:``}),V=i(!1),H=o({name:``,yaml:``}),U=i(!1),W=i(``),G=i(!1),K=i(``),q=i(``);async function J(){try{let e=await de({limit:50});e.code===0&&(R.value=e.data.items||[])}catch{}}async function ve(){if(!B.name)return;let e=await le(B);e.code===0&&(z.value=!1,B.name=``,B.description=``,e.data?.id?m.push(`/scenes/${e.data.id}`):J())}async function ye(e){q.value=e,K.value=`确定要删除该场景吗？此操作不可撤销。`,G.value=!0}async function be(){let e=q.value;G.value=!1,q.value=``,await ue(e),J()}function xe(e){m.push(`/scenes/${e.id}`)}function Se(){H.yaml=L,H.name=``}function Ce(){H.yaml=_e,H.name=``}async function we(){if(W.value=``,!H.yaml.trim()){W.value=`请输入 YAML 内容`;return}U.value=!0;try{let e=await ce({name:H.name,yaml:H.yaml});e.code===0?(V.value=!1,H.name=``,H.yaml=``,J(),e.data?.id&&m.push(`/scenes/${e.data.id}`)):W.value=e.message||`导入失败`}catch(e){W.value=e.message||`导入失败`}finally{U.value=!1}}function Y(e){if(!e)return`-`;let t=new Date(e),n=e=>String(e).padStart(2,`0`);return`${t.getFullYear()}-${n(t.getMonth()+1)}-${n(t.getDate())} ${n(t.getHours())}:${n(t.getMinutes())}:${n(t.getSeconds())}`}function X(e){if(!e)return`-`;let t=new Date(e),n=e=>String(e).padStart(2,`0`);return`${t.getFullYear()}-${n(t.getMonth()+1)}-${n(t.getDate())} ${n(t.getHours())}:${n(t.getMinutes())}:${n(t.getSeconds())}`}let Z=i(new Map);async function Te(){try{let{dashboardOverview:e}=await se(async()=>{let{dashboardOverview:e}=await import(`./dashboard-112_7TEr.js`);return{dashboardOverview:e}},__vite__mapDeps([0,1,2])),t=await e(86400*7);if(t.code===0&&t.data?.recent_runs){let e=new Map;t.data.recent_runs.forEach(t=>{let n=String(t.scene_id),r={started_at:t.started_at,finished_at:t.finished_at,status:t.status,duration:t.duration};e.has(n)||e.set(n,[]),e.get(n).push(r)}),Z.value=e}}catch(e){console.error(`Failed to fetch scene runs:`,e)}}function Q(e){let t=Z.value.get(e.id);if(!(!t||t.length===0))return t[t.length-1]}function $(e){return Q(e)?.status===`running`}function Ee(e){let t=Q(e);if(!t?.started_at)return`-`;let n=new Date(t.started_at).getTime(),r=(t.status===`running`?Date.now():t.finished_at?new Date(t.finished_at).getTime():Date.now())-n;if(r<=0)return`-`;let i=Math.floor(r/1e3),a=Math.floor(i/3600),o=Math.floor(i%3600/60),s=i%60,c=e=>String(e).padStart(2,`0`);return a>0?`${c(a)}小时${c(o)}分${c(s)}秒`:o>0?`${c(o)}分${c(s)}秒`:`${c(s)}秒`}return e(()=>{J(),Te()}),(e,i)=>{let o=ne(`router-link`);return a(),d(`div`,fe,[l(`div`,pe,[i[12]||=l(`h2`,null,`场景管理`,-1),l(`div`,me,[l(`button`,{class:`btn-secondary`,onClick:i[0]||=e=>V.value=!0},`导入 YAML`),l(`button`,{class:`btn-login-primary`,onClick:i[1]||=e=>z.value=!0},`+ 新建场景`)])]),l(`div`,he,[l(`table`,ge,[i[14]||=l(`thead`,null,[l(`tr`,null,[l(`th`,null,`ID`),l(`th`,null,`名称`),l(`th`,null,`描述`),l(`th`,null,`状态`),l(`th`,null,`创建时间`),l(`th`,null,`测试开始时间`),l(`th`,null,`结束时间`),l(`th`,null,`持续时间`),l(`th`,null,`操作`)])],-1),l(`tbody`,null,[R.value.length===0?(a(),d(`tr`,h,[...i[13]||=[l(`td`,{colspan:`9`,class:`empty`},`暂无场景`,-1)]])):u(``,!0),(a(!0),d(ie,null,t(R.value,e=>(a(),d(`tr`,{key:e.id},[l(`td`,g,c(e.id),1),l(`td`,null,[te(o,{to:`/scenes/${e.id}`,class:`link`},{default:n(()=>[ee(c(e.name),1)]),_:2},1032,[`to`])]),l(`td`,null,[l(`div`,{class:`desc-cell`,title:e.description},c(e.description||`-`),9,_)]),l(`td`,null,[l(`span`,{class:s([`status-badge`,e.status])},c(e.status),3)]),l(`td`,null,c(Y(e.created_at)),1),l(`td`,v,c(Q(e)?.started_at?X(Q(e).started_at):`-`),1),l(`td`,y,c(Q(e)?.finished_at?X(Q(e).finished_at):$(e)?`--`:`-`),1),l(`td`,b,c(Ee(e)),1),l(`td`,x,[l(`button`,{class:s([`btn-sm`,{disabled:$(e)}]),disabled:$(e),onClick:t=>xe(e)},`编辑`,10,S),l(`button`,{class:`btn-sm danger`,onClick:t=>ye(e.id)},`删除`,8,C)])]))),128))])])]),z.value?(a(),d(`div`,{key:0,class:`modal-overlay`,onClick:i[5]||=p(e=>z.value=!1,[`self`])},[l(`div`,w,[i[17]||=l(`h3`,null,`新建场景`,-1),l(`div`,T,[i[15]||=l(`label`,null,`名称`,-1),r(l(`input`,{"onUpdate:modelValue":i[2]||=e=>B.name=e,placeholder:`场景名称`},null,512),[[f,B.name]])]),l(`div`,E,[i[16]||=l(`label`,null,`描述`,-1),r(l(`input`,{"onUpdate:modelValue":i[3]||=e=>B.description=e,placeholder:`场景描述`},null,512),[[f,B.description]])]),l(`div`,D,[l(`button`,{class:`btn-secondary`,onClick:i[4]||=e=>z.value=!1},`取消`),l(`button`,{class:`btn-login-primary`,onClick:ve},`创建`)])])])):u(``,!0),G.value?(a(),d(`div`,{key:1,class:`modal-overlay`,onClick:i[7]||=p(e=>G.value=!1,[`self`])},[l(`div`,O,[i[18]||=ae(`<div class="confirm-icon" data-v-94eb0f24><svg width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" data-v-94eb0f24><circle cx="12" cy="12" r="10" data-v-94eb0f24></circle><line x1="12" y1="8" x2="12" y2="12" data-v-94eb0f24></line><line x1="12" y1="16" x2="12.01" y2="16" data-v-94eb0f24></line></svg></div><h3 class="confirm-title" data-v-94eb0f24>确认删除</h3>`,2),l(`p`,k,c(K.value),1),l(`div`,A,[l(`button`,{class:`btn-cancel`,onClick:i[6]||=e=>G.value=!1},`取消`),l(`button`,{class:`btn-danger-confirm`,onClick:be},`确认删除`)])])])):u(``,!0),V.value?(a(),d(`div`,{key:2,class:`modal-overlay`,onClick:i[11]||=p(e=>V.value=!1,[`self`])},[l(`div`,j,[i[21]||=l(`h3`,null,`导入 YAML 配置`,-1),i[22]||=l(`p`,{class:`import-hint`},`粘贴 YAML 配置内容，系统将自动创建场景和 DAG 请求流节点。支持 setup/teardown 生命周期、参数生成器 (generator)、参数关联 (extract)、条件判断 (condition) 和延迟 (delay) 等节点类型。可点击"加载示例"查看完整电商全链路配置模板。`,-1),l(`div`,M,[i[19]||=l(`label`,null,`场景名称`,-1),r(l(`input`,{"onUpdate:modelValue":i[8]||=e=>H.name=e,placeholder:`留空则使用 YAML 中的 name 字段`},null,512),[[f,H.name]])]),l(`div`,N,[i[20]||=l(`label`,null,`YAML 内容`,-1),r(l(`textarea`,{"onUpdate:modelValue":i[9]||=e=>H.yaml=e,class:`yaml-input`,placeholder:`在此粘贴 YAML 配置...`,rows:`24`},null,512),[[f,H.yaml]])]),W.value?(a(),d(`div`,P,c(W.value),1)):u(``,!0),l(`div`,F,[l(`button`,{class:`btn-secondary`,onClick:Se},`加载示例`),l(`button`,{class:`btn-secondary`,onClick:Ce},`加载新版示例`),l(`button`,{class:`btn-secondary`,onClick:i[10]||=e=>V.value=!1},`取消`),l(`button`,{class:`btn-login-primary`,onClick:we,disabled:U.value},c(U.value?`导入中...`:`导入`),9,I)])])])):u(``,!0)])}}}),[[`__scopeId`,`data-v-94eb0f24`]]);export{R as default};