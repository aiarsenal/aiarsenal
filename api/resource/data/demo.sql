-- 演示种子（F-002）：101 个演示账号（hi + user001..user100）+ 岗位绑定（100 条
-- aa_user_post）+ 角色绑定（100 条 g 规则）。仅显式声明演示环境时由 deploy.sh
-- rebuild-db 加载：
--   AIARSENAL_DEMO=true sudo bash deploy.sh rebuild-db
-- 演示账号口令为公开默认值 aiarsenal123，只可用于完全隔离的演示环境；
-- 生产部署不加载本文件（默认安装零种子，首个超管由 api 启动引导生成随机口令）。

COPY public.aa_user (id, user_id, user_name, employee_no, name, mobile, user_nickname, birthday, user_password, user_salt, user_status, user_email, sex, avatar, dept_id, remark, is_admin, address, describe, last_login_ip, last_login_time, created_at, updated_at, deleted_at, password_changed_at, totp_secret, totp_enabled) FROM stdin;
2	i47dhcwyx2rulbjl5vlab89orpamongb	user001	EMP00002	测试用户001	13910000001	测试用户001	0	$pbkdf2-sm3$60000$fBANdqzQqJhhChBFNcWL4w$20InWnM4/sBDJhZYsh1QRJVb1tdwt5gI3s5gT9sGCRk	z6kEm52pMp	1	user001@aiarsenal.cc	1		lvyrsk0fjk7br36yh0ti1fug3c8oewsz		0				2026-07-04 00:00:00	2026-07-04 00:00:00	2026-07-04 00:00:00	\N	\N		0
3	awaoolltywg611faqbrfjs0a6qq7e7mq	user002	EMP00003	测试用户002	13910000002	测试用户002	0	$pbkdf2-sm3$60000$0dC32faPSWnH7wTbUOYc0A$ZDh8oBbBrUbP/yx3wIgGcehXylCA+tCHLXEBw1Khf40	YUJkVpBgoq	1	user002@aiarsenal.cc	2		yvd2rzcq3p6on80czv6q7955j7gc5y3t		0				2026-07-04 00:00:00	2026-07-04 00:00:00	2026-07-04 00:00:00	\N	\N		0
4	ttozve8tomu4bw7q64iqch473vic8xjx	user003	EMP00004	测试用户003	13910000003	测试用户003	0	$pbkdf2-sm3$60000$N1iNLG0HLIDsJcDyowZnPA$WcbB9j+WhXUtBpsqGRveeNPcnPxzjOWSgsmcY8GzIdo	6vzTnE2ajm	1	user003@aiarsenal.cc	0		oyy8hxpdfp1b5blfpq8or6tk15hvhwwk		0				2026-07-04 00:00:00	2026-07-04 00:00:00	2026-07-04 00:00:00	\N	\N		0
5	l7v7hxinitk68gtbwp1ygd1ud39q88bt	user004	EMP00005	测试用户004	13910000004	测试用户004	0	$pbkdf2-sm3$60000$xK4nrjG07xIVkh7TpgVpow$rir63o30Stl4QQT2P7UgAvYbNdRscZwpsDNFKXkNxRM	Q0yMQMGbWO	1	user004@aiarsenal.cc	1		g84a2esunq3ggigh6rie4iso5mvqejoc		0				2026-07-04 00:00:00	2026-07-04 00:00:00	2026-07-04 00:00:00	\N	\N		0
6	g1zzvvdxqrpmzk2kw8q4ky3a26e6qfxv	user005	EMP00006	测试用户005	13910000005	测试用户005	0	$pbkdf2-sm3$60000$4pIUJ4iZd8KYeHtd+RErnA$iQ1eoZ8R2e9LMpNzaGQ8PFLU5FtSjg56nugTxOlDw7A	WqwfW8MGX6	1	user005@aiarsenal.cc	2		nfnkkl3va2fdvuk9fwmiz21y15xpl8lh		0				2026-07-04 00:00:00	2026-07-04 00:00:00	2026-07-04 00:00:00	\N	\N		0
7	wcsj3t9d38o5o5ryzp9pl0s72bbolqje	user006	EMP00007	测试用户006	13910000006	测试用户006	0	$pbkdf2-sm3$60000$nfS3Lpn7PhNGBwGMbaihxQ$/QJMB0q8vn6kT6tqVBAh4kXuTcyscJGtB4Mx33x27PU	hUSQIl8jD6	1	user006@aiarsenal.cc	0		v5r1ilh98qyl0kwnsrfr79whbyy1twzf		0				2026-07-04 00:00:00	2026-07-04 00:00:00	2026-07-04 00:00:00	\N	\N		0
8	psnpiaisc4utvtowod2cismxnen0sqji	user007	EMP00008	测试用户007	13910000007	测试用户007	0	$pbkdf2-sm3$60000$kCh/r3Q7dYKLsGoezjFPUA$axF548pQtrD1SFaTRiNFzafvCo1XAzdEj1VD2J7zKG8	zk968uweGR	1	user007@aiarsenal.cc	1		fnrcvzwdge0dwbwjaeyilydchrlmff9g		0				2026-07-04 00:00:00	2026-07-04 00:00:00	2026-07-04 00:00:00	\N	\N		0
9	yftk7uwwhy2oxmrzkq3tp4dh89bch1jb	user008	EMP00009	测试用户008	13910000008	测试用户008	0	$pbkdf2-sm3$60000$zQZYdGqXkbS9QSe3yBggDQ$YEKFnPfqFmLUL8HuybJ0SE7DNZrSDGAY0o8/NaAHukE	8APZ6BJdEB	1	user008@aiarsenal.cc	2		n7c0xt6ksdqwia6gjrtebatzlzz4c2tb		0				2026-07-04 00:00:00	2026-07-04 00:00:00	2026-07-04 00:00:00	\N	\N		0
10	ey81ndf6zpobmaoxqiquhofjpu02z8ht	user009	EMP00010	测试用户009	13910000009	测试用户009	0	$pbkdf2-sm3$60000$RNAiyqEyeQLBBctvvt1yWQ$0eBtXSZzd2iDyWRBThd4nAlk80xuju66UMIPRpUDXLc	V4bse9bK3H	1	user009@aiarsenal.cc	0		qdei4bqdhl2s48229y716b7zmyl0yhov		0				2026-07-04 00:00:00	2026-07-04 00:00:00	2026-07-04 00:00:00	\N	\N		0
11	hhhnxqvutljctsgaoo780vo563bez55a	user010	EMP00011	测试用户010	13910000010	测试用户010	0	$pbkdf2-sm3$60000$Hcs1P8DNlaT/ORwM6ohZcg$+pH1+BAvMYK5po+ahBISsZ72Bn/Zse51l4F384Is6Pw	pzQapNDT2q	1	user010@aiarsenal.cc	1		gzbhisqghr688sew022riq0m9ph1tnul		0				2026-07-04 00:00:00	2026-07-04 00:00:00	2026-07-04 00:00:00	\N	\N		0
12	xdrjswzjeydx1v5ge05jltxd0ixg0yya	user011	EMP00012	测试用户011	13910000011	测试用户011	0	$pbkdf2-sm3$60000$jidYW4U9E/ZBZQb1Xypw7A$LB67cWdW7v066GltxJ88sXbDyanoPcN1cr78itkU23E	FZ1JVZu5cg	1	user011@aiarsenal.cc	2		lcx6976nybie9legnbf7efda17gzms8p		0				2026-07-04 00:00:00	2026-07-04 00:00:00	2026-07-04 00:00:00	\N	\N		0
13	qqzy7xff86q8kfmyopjq5dmgbwf03lkd	user012	EMP00013	测试用户012	13910000012	测试用户012	0	$pbkdf2-sm3$60000$C9r1kRt9Yr8WIA39OZRSCA$i8EUTNQxRIrN5RYbOeKHr/cyjwmDf7Zui00cSBHjmfU	91v3TPoiEk	1	user012@aiarsenal.cc	0		tuon2vwblsnov4mnykch7sypzbvwgqma		0				2026-07-04 00:00:00	2026-07-04 00:00:00	2026-07-04 00:00:00	\N	\N		0
14	hwogmz6ku42vzbxo7l4g9seiftxmf55z	user013	EMP00014	测试用户013	13910000013	测试用户013	0	$pbkdf2-sm3$60000$1xLQ8c3egJy3mis27Qnh2Q$xi1f7yVbqNX9By6c15Ig6eehD9clqjkdZSWTa8PEPbE	Cy2cM9MWjF	1	user013@aiarsenal.cc	1		v82stxipz7d1bdq1iz4ovthbs019tphi		0				2026-07-04 00:00:00	2026-07-04 00:00:00	2026-07-04 00:00:00	\N	\N		0
15	vxxrlq1k4nmwnbuiud4z8g4zdn6brnom	user014	EMP00015	测试用户014	13910000014	测试用户014	0	$pbkdf2-sm3$60000$ZAU8IRvyJOFT1zvSHQs0dw$4vYhc3YuyyJI31WquKX8Y9YYE/VE+lhGj91JaHpf3uc	RzZKHhVFJJ	1	user014@aiarsenal.cc	2		kheqge89r0euopogj346fyg25m8t3rya		0				2026-07-04 00:00:00	2026-07-04 00:00:00	2026-07-04 00:00:00	\N	\N		0
16	t8ubxdsaiuixyj57rt57vhrggh6yejco	user015	EMP00016	测试用户015	13910000015	测试用户015	0	$pbkdf2-sm3$60000$9fQApxzCuvArLAocbN+COw$skIwFd0RdrW4rtBH9hMaso+4rp+gZb+tihibI0Vt5WE	w5fkk08OMa	1	user015@aiarsenal.cc	0		fop8s0srp4a6or6ceam5nyp6632unzbg		0				2026-07-04 00:00:00	2026-07-04 00:00:00	2026-07-04 00:00:00	\N	\N		0
17	huoc4rxc0p8g6f4cswatmogr6daqfxgl	user016	EMP00017	测试用户016	13910000016	测试用户016	0	$pbkdf2-sm3$60000$Qmi93OFd2qJlGRsUgFQgdA$RWmYwU+0vCCllWotYO9RA593tCCvuTnLO8QcVuCwACo	71R6YZgK3C	1	user016@aiarsenal.cc	1		hbxawf9h3d7xkyjmbw4uqeon4i3nfn2w		0				2026-07-04 00:00:00	2026-07-04 00:00:00	2026-07-04 00:00:00	\N	\N		0
18	yv6887f2ne6v8cwx0shm2ty97nbv4vme	user017	EMP00018	测试用户017	13910000017	测试用户017	0	$pbkdf2-sm3$60000$eGh28UX9kFVTIkpOF809Aw$iTu4tZXXvPhH9n2n1uuiGR4tJCFkBP/b2sBeqpMK3oA	k6HYFmCrWk	1	user017@aiarsenal.cc	2		ivgstgmxznc1erf0sxvtxjqfikq7wazs		0				2026-07-04 00:00:00	2026-07-04 00:00:00	2026-07-04 00:00:00	\N	\N		0
19	cz5dv5ogq1ifcqmsempf00hqx4w9e7fr	user018	EMP00019	测试用户018	13910000018	测试用户018	0	$pbkdf2-sm3$60000$vLzl+FgcKCyFqXyuaPIpTA$33/XoF4r5UwYntJmQQeFLuGxJYur1hY5mmZ3pOE+fkY	uGY3kBxc4J	1	user018@aiarsenal.cc	0		kye5pyfqoq5doipzenotptllr2g6mxqg		0				2026-07-04 00:00:00	2026-07-04 00:00:00	2026-07-04 00:00:00	\N	\N		0
20	ddv2o2ggneyz5omf2hsakq5gq5nb37kp	user019	EMP00020	测试用户019	13910000019	测试用户019	0	$pbkdf2-sm3$60000$Up0LUKk7Yq1E/3NwJbKqmQ$wo5wSdBwb6vfgGFAZ9UBvNX0lCZJv5cqxgYlW7tc3yA	6idosZzNt9	1	user019@aiarsenal.cc	1		zujehiadtnjaqtgooixca2iqp9safihi		0				2026-07-04 00:00:00	2026-07-04 00:00:00	2026-07-04 00:00:00	\N	\N		0
21	iozdijm6umoh07mwmf9shtyb3gkm6b2e	user020	EMP00021	测试用户020	13910000020	测试用户020	0	$pbkdf2-sm3$60000$SrPgbJPal1LZlPibpOwI8w$hZszwwW4FLb7o3lgkToobRdOLvKGITVz0VfihvJ6Rmo	eLIqrmVWTp	1	user020@aiarsenal.cc	2		eow0qt4tkj5qw2k56wi30oupnb3yct4m		0				2026-07-04 00:00:00	2026-07-04 00:00:00	2026-07-04 00:00:00	\N	\N		0
22	xkbw9qo0volhjszdrgr11ieh9qmy05xm	user021	EMP00022	测试用户021	13910000021	测试用户021	0	$pbkdf2-sm3$60000$4744UQB/3dlpcIKqu+b+yQ$ubqFtD8LYFxh1eQ5IdIlNloJ7Kg8hJRo0AG4/wIN9gU	21eIfppwZb	1	user021@aiarsenal.cc	0		eleqfwc2744a9ozwn5ndpayuuw7c4zqy		0				2026-07-04 00:00:00	2026-07-04 00:00:00	2026-07-04 00:00:00	\N	\N		0
23	ojmwz7cbldpxwnmu4h9ed5gzjm8yglbj	user022	EMP00023	测试用户022	13910000022	测试用户022	0	$pbkdf2-sm3$60000$/Qdc+s/1mPULrj60Gnn/CA$hA/qe84NN5eRYNkdnk81UapZhWHERz3t5GXPQv4+Mio	3ePZt6vwhc	1	user022@aiarsenal.cc	1		llxllcao3wked14skgm1lb5i7p11a5jt		0				2026-07-04 00:00:00	2026-07-04 00:00:00	2026-07-04 00:00:00	\N	\N		0
24	ouvzpmuuf8vmoja8g7ua9x1a6m83dkqe	user023	EMP00024	测试用户023	13910000023	测试用户023	0	$pbkdf2-sm3$60000$oI9h9LD4L9LqKijyr6LQYg$doRRgTP8z1m/3r6gNRgNA7Tmc2ArErroeYqg7vJYKoI	3MujzyroRE	1	user023@aiarsenal.cc	2		cyrj194puwursqce38vm471isql0qc4j		0				2026-07-04 00:00:00	2026-07-04 00:00:00	2026-07-04 00:00:00	\N	\N		0
25	f571jz7rptodss6igenpqkbdv9s8nhgq	user024	EMP00025	测试用户024	13910000024	测试用户024	0	$pbkdf2-sm3$60000$4MMM7YVB5wIicYhAafL/0w$D/e8QVL3M/bKoaBO+hzDkCi+2kFyoUhL/iflgHRtcf0	yfsd5styKR	1	user024@aiarsenal.cc	0		m2hu15ojpus1layz4k4zhifi9cj8erbt		0				2026-07-04 00:00:00	2026-07-04 00:00:00	2026-07-04 00:00:00	\N	\N		0
26	nn5sxc1va1wbi7c3y8nj3si1tznkil1o	user025	EMP00026	测试用户025	13910000025	测试用户025	0	$pbkdf2-sm3$60000$wOoRQ6OV0iyNywgosymZ3Q$gcGHyGL3dB3NQxxllxbSDUbzeQp4LIC2M846+6XIDp0	GYgdqZi99n	1	user025@aiarsenal.cc	1		b52vpvw2wompdleclbp5467pkw017oab		0				2026-07-04 00:00:00	2026-07-04 00:00:00	2026-07-04 00:00:00	\N	\N		0
27	rjtjyaegy2r498hswx8htls1ncqz2shp	user026	EMP00027	测试用户026	13910000026	测试用户026	0	$pbkdf2-sm3$60000$Q1TLLi34Y++f6DVJccUFrg$dsoMdSklDL4LFx4E0NjON7P+Mui9wwCeYrTnWbr+EVA	kEx6LJtYIM	1	user026@aiarsenal.cc	2		gb2wz5r8etns12cgiu4pi627n2sqqlfw		0				2026-07-04 00:00:00	2026-07-04 00:00:00	2026-07-04 00:00:00	\N	\N		0
28	o9ryfxtbs7mz7d1pitt1ji1eyofbuvhl	user027	EMP00028	测试用户027	13910000027	测试用户027	0	$pbkdf2-sm3$60000$FyVbufYcjfrmI9aWnOM5oA$ZpYNnHY9TPqUyUHbZxYM415K1z/XpOKPY0vpTHBNC6E	jDcuTq7ozs	1	user027@aiarsenal.cc	0		tm1yja3iahb1f4v3bhc7q7amvcu9crhk		0				2026-07-04 00:00:00	2026-07-04 00:00:00	2026-07-04 00:00:00	\N	\N		0
29	v8nyq4ekvn96ffe69fpteqprfbyw7d5o	user028	EMP00029	测试用户028	13910000028	测试用户028	0	$pbkdf2-sm3$60000$DrxvPYhRWk+C1KDrjYKOJA$qthIqCf+6cBk/Qa1o2WEamd3NO2AKgeazLwjbGwyQuA	Uo3AqHFJo1	1	user028@aiarsenal.cc	1		tekf85gn813frzxhmyy3yaj6j0x4e1fo		0				2026-07-04 00:00:00	2026-07-04 00:00:00	2026-07-04 00:00:00	\N	\N		0
30	ccp94mq5kpf7del1n8kixhght9hoc74f	user029	EMP00030	测试用户029	13910000029	测试用户029	0	$pbkdf2-sm3$60000$MFTRvLM+n1Ma/V0g6gM5fA$Vtl3viB39ejdsFD+oe+/TBWrC94viyDj4D/G53lay9M	dcjmCpOe62	1	user029@aiarsenal.cc	2		n59qw5rkvekax5p3m1ahoxmf1htfy42k		0				2026-07-04 00:00:00	2026-07-04 00:00:00	2026-07-04 00:00:00	\N	\N		0
31	i5jq4vt4wk4pf3lb4nx2fh4339wbfz0y	user030	EMP00031	测试用户030	13910000030	测试用户030	0	$pbkdf2-sm3$60000$8JfHrzkD4C6AMj3M+Ji/zg$AK+0J4lZqDz+fLIshX2QYcTApHbagZZoFyqgq7QWHNo	bYpZ909avt	1	user030@aiarsenal.cc	0		hwjzw8gpww9l4eqrvgfr729lk2o3lvzf		0				2026-07-04 00:00:00	2026-07-04 00:00:00	2026-07-04 00:00:00	\N	\N		0
32	h30ls46rppecsk1di4n5zxlz7xpcps5i	user031	EMP00032	测试用户031	13910000031	测试用户031	0	$pbkdf2-sm3$60000$tqh7qQ4qg8I3Mi89WdSE7w$wWmkFa6VK2en0F6wLV2AduOX0tm+WOyRjIDHTJic8Es	mlk3c0S92B	1	user031@aiarsenal.cc	1		oouqcfyiwgzfx9m1lcm6fx21ef3jzhkl		0				2026-07-04 00:00:00	2026-07-04 00:00:00	2026-07-04 00:00:00	\N	\N		0
33	g4dp1tsgez5vvn1a2hq7vpgrireuwr3f	user032	EMP00033	测试用户032	13910000032	测试用户032	0	$pbkdf2-sm3$60000$YURjprmXznClZ/I5DOPcow$T5jdRRbzLzl78sgaEe1OcLngwtUu5E3KiXV5U6hbMmQ	43bR34SUpA	1	user032@aiarsenal.cc	2		jxtny36vgipqmo8dorsquvfonx2bitdm		0				2026-07-04 00:00:00	2026-07-04 00:00:00	2026-07-04 00:00:00	\N	\N		0
34	xh9e2z238dalqjxse0gdlrteaf49325f	user033	EMP00034	测试用户033	13910000033	测试用户033	0	$pbkdf2-sm3$60000$5B6qdmXz15K/2Uvv6UjUxw$pg5jmlpwkIDjnEp8ItO0xzpI86mIt3jgxuD7tYNBWbo	4hBrlzcvWs	1	user033@aiarsenal.cc	0		p993upszgt1evrgcpxv7f910ixu2sbgs		0				2026-07-04 00:00:00	2026-07-04 00:00:00	2026-07-04 00:00:00	\N	\N		0
35	p6k1ds5468pmzbz1i6v4814lt5475m3m	user034	EMP00035	测试用户034	13910000034	测试用户034	0	$pbkdf2-sm3$60000$ApBOXPYpNHidAWcdePRPQg$w2givzt/TDd8BqWOUyIwWSC6oHt+s96wa7iI8cx6mG8	uJzT4RVSWy	1	user034@aiarsenal.cc	1		azohlyb3og2beqrj9kpn96sakc44dadf		0				2026-07-04 00:00:00	2026-07-04 00:00:00	2026-07-04 00:00:00	\N	\N		0
36	nxbyzuyiu6ouyygcuzemmac873aljn3j	user035	EMP00036	测试用户035	13910000035	测试用户035	0	$pbkdf2-sm3$60000$jzJnj0N1YVDfs2oK6ujMHg$NWb60qZ33md9XWe/OqzHszU6waSpY0gDOs4Ln9ogImM	ctbzcGNJUA	1	user035@aiarsenal.cc	2		aeqjdjfs1wyl9p93u8bayg352ztdm7ya		0				2026-07-04 00:00:00	2026-07-04 00:00:00	2026-07-04 00:00:00	\N	\N		0
37	f386t32hk8sghcnvf24g3st3i41phl9o	user036	EMP00037	测试用户036	13910000036	测试用户036	0	$pbkdf2-sm3$60000$kE8tNTTlRwPzJ750VoJQAw$x5FF8808Uj0FW8CgtQKNqOJgQROT99d4VfUW+YBaBWE	J7d6lCF6KZ	1	user036@aiarsenal.cc	0		zclzz8og8qqe6kebyzszab5g2ufrigzm		0				2026-07-04 00:00:00	2026-07-04 00:00:00	2026-07-04 00:00:00	\N	\N		0
38	qu9owum1ui0elni61b5i29akyl7jt03p	user037	EMP00038	测试用户037	13910000037	测试用户037	0	$pbkdf2-sm3$60000$KV0gOWPrCMyFBz2nnpX/WQ$BcTUUyjwidFb/1AmCZuf3eoSbmIpUw2ijpA/7GupcgI	tDugPpyeQb	1	user037@aiarsenal.cc	1		kdipud5mow01mfleoyegyrf9nzlh0omp		0				2026-07-04 00:00:00	2026-07-04 00:00:00	2026-07-04 00:00:00	\N	\N		0
39	b7qp8p0xj1okbz6vxrmzngd32jsonytp	user038	EMP00039	测试用户038	13910000038	测试用户038	0	$pbkdf2-sm3$60000$G9dGkk3BVjA6+flhuahDYw$CiOowI6bMLYkyHj5dUEowrzhD2V7ml7XT5qDK19PHPU	mOUAetSJ7M	1	user038@aiarsenal.cc	2		c4psurjud21mvvvpmuelaf1rt7gfjnno		0				2026-07-04 00:00:00	2026-07-04 00:00:00	2026-07-04 00:00:00	\N	\N		0
40	qh1et5ylq2fm1e82d32fl67gvnqo6riq	user039	EMP00040	测试用户039	13910000039	测试用户039	0	$pbkdf2-sm3$60000$9PtM8b5GVaWyQrPA8axSPg$fuJB1XXrJjrjwVFdfEawXsRcBPR2tovm6Ero7I0ORsI	WhdSEsjchL	1	user039@aiarsenal.cc	0		lhq17wpyf7oh04funia9ki4t2tblbq6r		0				2026-07-04 00:00:00	2026-07-04 00:00:00	2026-07-04 00:00:00	\N	\N		0
41	tyrhx8gab3hzay0xmz65aia5j5xzjcuo	user040	EMP00041	测试用户040	13910000040	测试用户040	0	$pbkdf2-sm3$60000$Q9e3C9H4CnoMpV968FxLlA$wE0Bg9IXaO8MAt/UrdIYOG0Taas60cvhUWBR/ICu0cw	r9A9OJvyF6	1	user040@aiarsenal.cc	1		pw2cp1u6l37m0h9vrnlgx8aq4ddnelxi		0				2026-07-04 00:00:00	2026-07-04 00:00:00	2026-07-04 00:00:00	\N	\N		0
42	wlkajsoyh27oscyja111i6bq5e5cd5gx	user041	EMP00042	测试用户041	13910000041	测试用户041	0	$pbkdf2-sm3$60000$4IqJxxniJ7bRdbC5yOzgVw$fVSrmORxd+bcOgvZgS6RjfZ2bkhO6sjZ1zDBZBUitA4	nQddHT2S4b	1	user041@aiarsenal.cc	2		bgnoctfgtiezihbpp2xjmzh55134eykr		0				2026-07-04 00:00:00	2026-07-04 00:00:00	2026-07-04 00:00:00	\N	\N		0
43	ki2q6fgdt8o56u3vn2ckanvoy34qqq9a	user042	EMP00043	测试用户042	13910000042	测试用户042	0	$pbkdf2-sm3$60000$y+gFN9Nq55HfvLs0vBqF9g$KpyVQ6s2z26QMluCNyaXzZg2H5zCUoxhIZPbd3IN9Cs	6BPK5fJY9M	1	user042@aiarsenal.cc	0		jqtjxsygdgv7btkrb6sfau7fp8coeqgz		0				2026-07-04 00:00:00	2026-07-04 00:00:00	2026-07-04 00:00:00	\N	\N		0
44	lsinkrec95cl8xk41pc1xa0nwp77r7nq	user043	EMP00044	测试用户043	13910000043	测试用户043	0	$pbkdf2-sm3$60000$lt0rN0FaVAVUYWUclA9r2Q$aMjYnMcmavHOMtoamHTxkRa4HO4sUBZUbPkENUdXnZc	RDTxwgMLxm	1	user043@aiarsenal.cc	1		megotp94fof0d26f3g6zj3xg2lgsuwut		0				2026-07-04 00:00:00	2026-07-04 00:00:00	2026-07-04 00:00:00	\N	\N		0
45	u7keg12zsf1fp6wxnd0w78r03lk9p5ne	user044	EMP00045	测试用户044	13910000044	测试用户044	0	$pbkdf2-sm3$60000$kkVquvog/YxckNyEOjRung$w89uitU9jMfZY1mZM2coL1wsIpmghQvTRu1iy8Q4PBs	8MWEPCAUUf	1	user044@aiarsenal.cc	2		r7haexv1ewuo2e40uh4tnx5zxxmqnfns		0				2026-07-04 00:00:00	2026-07-04 00:00:00	2026-07-04 00:00:00	\N	\N		0
46	vj9e202tchk0m7ar1pg1w2xvcb52770u	user045	EMP00046	测试用户045	13910000045	测试用户045	0	$pbkdf2-sm3$60000$i1IqiRMPqCQE6frIxmlGBw$p4e+tpsJTX+ymtSbJQj69+ceD8XUJdWpiCca4CNVB0M	LRzOp1kb4P	1	user045@aiarsenal.cc	0		u1ai4dx2jxsimrdtb0q22n1p6ie71z8b		0				2026-07-04 00:00:00	2026-07-04 00:00:00	2026-07-04 00:00:00	\N	\N		0
47	bdlps6p632yb96o44b00s23l1eywr0sy	user046	EMP00047	测试用户046	13910000046	测试用户046	0	$pbkdf2-sm3$60000$R+b5uQNauGcyVCxxgp0P9w$Sk17AJezwLJ2bx5mbqZMSrH6JhPQhUqEmZSjNRWB9G8	DNN63VvJWw	1	user046@aiarsenal.cc	1		in2z18l0hxz17jljs3h3qckjtq7q1kti		0				2026-07-04 00:00:00	2026-07-04 00:00:00	2026-07-04 00:00:00	\N	\N		0
48	fd5893szdczhr9ylo1cwbrsbb7f6s0np	user047	EMP00048	测试用户047	13910000047	测试用户047	0	$pbkdf2-sm3$60000$UvsyEC1yzDXSDSSsy1ecig$YYasv1JX9pbBuN/9mJRWtG8h8P4WC0k743YTuSxvH/8	nFE9TfPX5r	1	user047@aiarsenal.cc	2		r650hfoqzv599kagklh6f836tto2rbbw		0				2026-07-04 00:00:00	2026-07-04 00:00:00	2026-07-04 00:00:00	\N	\N		0
49	fisec94xg1zmsoe73xqi8s7pcioxs2xy	user048	EMP00049	测试用户048	13910000048	测试用户048	0	$pbkdf2-sm3$60000$Au8rsMSGTRyBzhq+M/F/lg$67CNtbLMZ8WscFJPb/9ohq9klMhj+MpVBstoWjmg5e0	bMmU7Pji8y	1	user048@aiarsenal.cc	0		gj2z499cescrl02ierj1r3hhwzqwlqjo		0				2026-07-04 00:00:00	2026-07-04 00:00:00	2026-07-04 00:00:00	\N	\N		0
50	qjnllgil3gdfowp1khpin6o2b6egz7vs	user049	EMP00050	测试用户049	13910000049	测试用户049	0	$pbkdf2-sm3$60000$eaF8tVJByZz5Spf4/oJ1eQ$v/v2a8KBkrmbSzJRrnkB1xC4a5RpdXUDIVgTArUc86Q	zLazGKrlF2	1	user049@aiarsenal.cc	1		w5uych9iovhs7ytww2xyni9a3gvn9f9p		0				2026-07-04 00:00:00	2026-07-04 00:00:00	2026-07-04 00:00:00	\N	\N		0
51	et5k755mj9ndmzcz5uqj79o0se8a7hjq	user050	EMP00051	测试用户050	13910000050	测试用户050	0	$pbkdf2-sm3$60000$+UGu51z2igpQXwa6+mFNlg$u3oglQ/R2b8lDggs3gJojYdLa9wrjIGpVxKYy81dPMc	EvhUEjQItI	1	user050@aiarsenal.cc	2		r836v7gwal4mjozdm21vu2vhvp5tjoax		0				2026-07-04 00:00:00	2026-07-04 00:00:00	2026-07-04 00:00:00	\N	\N		0
52	z1cnqvbthuj6j2ocm19p5wivwknkwwxb	user051	EMP00052	测试用户051	13910000051	测试用户051	0	$pbkdf2-sm3$60000$5pWC8FvkZcPGZEmv8hZ9ZQ$beCJCdveLKQfyINAO3bD2aNYOiaB1sb5N0he9jZT0d4	W7jN03xcLr	1	user051@aiarsenal.cc	0		kzd9e9wccd2d1fyonp6ucvani848uqjq		0				2026-07-04 00:00:00	2026-07-04 00:00:00	2026-07-04 00:00:00	\N	\N		0
53	fgyo9wgof0kcalr2r8cskc2g0bejiw4r	user052	EMP00053	测试用户052	13910000052	测试用户052	0	$pbkdf2-sm3$60000$SPsUazjsaqvwjHo+MFPw2Q$hPRRWDDVEUxxb3w0SSy/L0OeZd/Vz06GWe7LZI8gSWU	Sps29ftxlx	1	user052@aiarsenal.cc	1		i1qwazesxbz321eroqn5o0y0i6ri5ucq		0				2026-07-04 00:00:00	2026-07-04 00:00:00	2026-07-04 00:00:00	\N	\N		0
54	zzye39s02viahffg8futr2nq2m8ahw9v	user053	EMP00054	测试用户053	13910000053	测试用户053	0	$pbkdf2-sm3$60000$vt/Q3I3EbhPpHBFAE/B8gw$h7/XIjV2tKtWs/hzd0h1xq7bqy14uKfjEY6it6DRXEA	fDbDrrwHvv	1	user053@aiarsenal.cc	2		huo1n8eyycgqr41cqhjoq3k3f5t4iapq		0				2026-07-04 00:00:00	2026-07-04 00:00:00	2026-07-04 00:00:00	\N	\N		0
55	nwesstx94ue6ekr832lgdj7zz1r81yol	user054	EMP00055	测试用户054	13910000054	测试用户054	0	$pbkdf2-sm3$60000$CiZAfp51i1LlByqRjiohuw$n1YlPIMzjgvkiiVLptSrKO7f9ZGMjxhYILl2dDJYZGg	fXgfnYSYwI	1	user054@aiarsenal.cc	0		mot73avfyez5gquc16nke4g4j6zmwrru		0				2026-07-04 00:00:00	2026-07-04 00:00:00	2026-07-04 00:00:00	\N	\N		0
56	fczzjf2lns76m9avq50s302n8bfv6kxy	user055	EMP00056	测试用户055	13910000055	测试用户055	0	$pbkdf2-sm3$60000$lNYaHuH1rZsjhuAnWKnsbA$IzvMN4pZ9ZuN6P2547dDF5wP4gRuCuaGUPo/pFYMoWM	H3c5MF7EhA	1	user055@aiarsenal.cc	1		xdt0wohr8eddgcpxfenx48iekck2rlyn		0				2026-07-04 00:00:00	2026-07-04 00:00:00	2026-07-04 00:00:00	\N	\N		0
57	ljkgwn4eowlvuh9aafzdc0ykhj06ptpw	user056	EMP00057	测试用户056	13910000056	测试用户056	0	$pbkdf2-sm3$60000$mEcjE+2+Vv8utQ9+TQsEXQ$HrIjX5MIX0Fivah5IriFOU4M3n1MGwNqa+1Tj/uJppM	PlpsEBmy6T	1	user056@aiarsenal.cc	2		vutmnr0p3uj6cwntn6x0fdypeckea2dp		0				2026-07-04 00:00:00	2026-07-04 00:00:00	2026-07-04 00:00:00	\N	\N		0
58	rbfrd3phrnqazpxz83q61q87t3s6znun	user057	EMP00058	测试用户057	13910000057	测试用户057	0	$pbkdf2-sm3$60000$ouF99TNFqBQnJ/PLNhAWug$V7h9xLcxLt2eb9qkcqPTAwxtOyCM1TolzOjUqTtln2A	MhbquN6iKj	1	user057@aiarsenal.cc	0		y8ckjvpi1fzt332kbs779wdylwwf919d		0				2026-07-04 00:00:00	2026-07-04 00:00:00	2026-07-04 00:00:00	\N	\N		0
59	qdroqjw2xx7ckth36lsw9lfmexyf1fbt	user058	EMP00059	测试用户058	13910000058	测试用户058	0	$pbkdf2-sm3$60000$YMcXKg8SmArwmdsoEdL1Og$V9ygK6HqnuwLFnuSS9yrcvwyoIpMeDCPnlQXqL1k3xA	tqbTiDe5bJ	1	user058@aiarsenal.cc	1		yyxgogstqncjk8aw1bka8ww8ql78t4oy		0				2026-07-04 00:00:00	2026-07-04 00:00:00	2026-07-04 00:00:00	\N	\N		0
60	nadp1hsoga0hqcw16pynzk19h7ehcj5r	user059	EMP00060	测试用户059	13910000059	测试用户059	0	$pbkdf2-sm3$60000$xuyFQNFOzzgeuQQgE04KFg$Q7hQ903SLwymU2WWWswftBjNNfScZCQJ0Cplpg3F2dU	HLPvs64Bzv	1	user059@aiarsenal.cc	2		aazaaxi3fj8ioshkfzfwd52wqn2bdvaq		0				2026-07-04 00:00:00	2026-07-04 00:00:00	2026-07-04 00:00:00	\N	\N		0
61	l5iu02zzxae7gw6denbs5m430pnakwco	user060	EMP00061	测试用户060	13910000060	测试用户060	0	$pbkdf2-sm3$60000$ZXd9FABJjMfznF+oKn/YAg$Bvilo/uTp7z7eH8d1dUUNIPGVNfx1msyXOcZ91zuLg8	S8QThD4Q0M	1	user060@aiarsenal.cc	0		rtcw7nc9vixl5y6u38ejdk6sisbngq4j		0				2026-07-04 00:00:00	2026-07-04 00:00:00	2026-07-04 00:00:00	\N	\N		0
62	g5zzcwa6lljcra9bh95ot0wmdbo8bb1v	user061	EMP00062	测试用户061	13910000061	测试用户061	0	$pbkdf2-sm3$60000$DBvSb2kZO5gWdDjW71yv2Q$c4KrcFrPSw6m+6luupuFUOIoRMWldYIKEOZGSAjBKTY	GMUobwhnon	1	user061@aiarsenal.cc	1		td7m81oe400y1y047drts97ljd1p9gdo		0				2026-07-04 00:00:00	2026-07-04 00:00:00	2026-07-04 00:00:00	\N	\N		0
63	idg18sc5jlnnbs02myk619oj5s8kstej	user062	EMP00063	测试用户062	13910000062	测试用户062	0	$pbkdf2-sm3$60000$8pu5Xv1l2IxVsR7CTgZXwA$xuPDMDKEGkbwNjckvMFAaR7YkX1fXkyPX11jQcwLp6Y	dAtVvXHNdT	1	user062@aiarsenal.cc	2		kzru6bn2yy8rmbss3l16blzo04uzf3qc		0				2026-07-04 00:00:00	2026-07-04 00:00:00	2026-07-04 00:00:00	\N	\N		0
64	ap2xvhdaf6feytr8bbfa0x563uuoztsm	user063	EMP00064	测试用户063	13910000063	测试用户063	0	$pbkdf2-sm3$60000$cF0E4+RR9FUsIDhoY0SFww$BpXZgHAWjAto43Eyw21PIzHxl+A6zsoIBcWC5oixMJ4	1TQtfZirpo	1	user063@aiarsenal.cc	0		ec0d5a12u547oegklqcfbx1ajrmhnhna		0				2026-07-04 00:00:00	2026-07-04 00:00:00	2026-07-04 00:00:00	\N	\N		0
65	aqjsus7aopdpz778evymkbst0mwg6s4l	user064	EMP00065	测试用户064	13910000064	测试用户064	0	$pbkdf2-sm3$60000$eBg3wS5Z0aw76Qth6nYsJg$KZ80PfKME+ClPhLSwBL9XOppI0uu+9M3flszribV2i8	bFevfiu9Xd	1	user064@aiarsenal.cc	1		m12ho3wliz43udlq3dhwmc6nu3wz9arh		0				2026-07-04 00:00:00	2026-07-04 00:00:00	2026-07-04 00:00:00	\N	\N		0
66	y9i1zy0zq9jtjx57eleglzfvurltdf9v	user065	EMP00066	测试用户065	13910000065	测试用户065	0	$pbkdf2-sm3$60000$2kT4BGO12hemc43na2TsUg$kfDyfQxWlBAv3edi3ZssWJXMMff83aGn5O0yZ+gs35E	6CPcFsAfcF	1	user065@aiarsenal.cc	2		ejt8axdoxjkpq2er6gk12yxfp6qsyvyh		0				2026-07-04 00:00:00	2026-07-04 00:00:00	2026-07-04 00:00:00	\N	\N		0
67	swy9yjech3it00fnpuk9m5t3ww50f0zu	user066	EMP00067	测试用户066	13910000066	测试用户066	0	$pbkdf2-sm3$60000$t819eq1nmW+ey5OA1YuvTw$RdjQKcA10TLnM1QIF1rhwO5vhaqioxDrZnxV5CEmPRk	rdhUvkmsFp	1	user066@aiarsenal.cc	0		sj8j1hw1likzmdbk4etbrqk5t6pnylle		0				2026-07-04 00:00:00	2026-07-04 00:00:00	2026-07-04 00:00:00	\N	\N		0
68	kp0b2zujdinw3quweijsa8k1fn5jo29l	user067	EMP00068	测试用户067	13910000067	测试用户067	0	$pbkdf2-sm3$60000$jF2xyZ3dp7E25waJ7jWwKg$9Qfz6eY47RgxtcgrD7gYZDnCx79frH2ABhsdqNspYp4	lDUnLSLANU	1	user067@aiarsenal.cc	1		vmxjerxj1mbh16efg08wykyqsw2ankvp		0				2026-07-04 00:00:00	2026-07-04 00:00:00	2026-07-04 00:00:00	\N	\N		0
69	t6bh2x83k6hg6haogt4loyl3dw283d1d	user068	EMP00069	测试用户068	13910000068	测试用户068	0	$pbkdf2-sm3$60000$Z47I3nGtW4vomjTGhWpD2w$u0B6ZjR2ZAghsT9/0US+Nlt9/fcSWyzuxBaYlpUgKpQ	TlwKCsjsUs	1	user068@aiarsenal.cc	2		cqu8wn24karw76b2d2kqluxoniq77mnh		0				2026-07-04 00:00:00	2026-07-04 00:00:00	2026-07-04 00:00:00	\N	\N		0
70	nk12a6f4keadmc12tfw69abqnb0hco5d	user069	EMP00070	测试用户069	13910000069	测试用户069	0	$pbkdf2-sm3$60000$xEg/I4dqOXvr8DeMF0z1Sw$u9CwobapG5CGLAeW4d+mzjT0IaPNjbpg0Dyy9j3tUu4	zPnVEXa8ep	1	user069@aiarsenal.cc	0		g1crjufoco9y6ylxn4tgaq5l13mn0isz		0				2026-07-04 00:00:00	2026-07-04 00:00:00	2026-07-04 00:00:00	\N	\N		0
71	zqb590opd1gatrzyadk6xax7e7bx500y	user070	EMP00071	测试用户070	13910000070	测试用户070	0	$pbkdf2-sm3$60000$SebwSoNZsN8T4etY0wLZMQ$NxSkmV+aTBcxItmSGfDFAF2rM56IW6A3h/A/CsZTyP4	pUxlDB9ZqN	1	user070@aiarsenal.cc	1		d3etjyk6js9jy8c0thmloppmoabyyxli		0				2026-07-04 00:00:00	2026-07-04 00:00:00	2026-07-04 00:00:00	\N	\N		0
72	hyztdi4uhb1w7ndfi47zpp1y5m16soai	user071	EMP00072	测试用户071	13910000071	测试用户071	0	$pbkdf2-sm3$60000$Ni2lffWBRL7GojythsGJrw$HXjf6szJ06Hegpzh/bIzZM19Lym7ylSk+AWD9lwWxwE	6tXSukPyGq	1	user071@aiarsenal.cc	2		q706ie5vt4qhfp6kl55fhwa7619x3ljj		0				2026-07-04 00:00:00	2026-07-04 00:00:00	2026-07-04 00:00:00	\N	\N		0
73	kb38c9j616dragwwgbyo8p7j4vk1xnub	user072	EMP00073	测试用户072	13910000072	测试用户072	0	$pbkdf2-sm3$60000$pFJXJJrcTMxlNhCJCvdmyA$3ehcUjZRF48t6fkdwfajjpDu/jqP1AQgQBmgV9/UUsU	opd2xQbMkq	1	user072@aiarsenal.cc	0		enc3p7e6phadqkpaoukpxn3qxaeojfou		0				2026-07-04 00:00:00	2026-07-04 00:00:00	2026-07-04 00:00:00	\N	\N		0
74	b0f6pm4d64m6zz6agzg7ff1y80kb112t	user073	EMP00074	测试用户073	13910000073	测试用户073	0	$pbkdf2-sm3$60000$kZBhG8XN69ilDutPFZcn4Q$V+6M6JoAiR46jUJGYCAggqDlGG/juluLEM9M2yQZ5KY	ZPKrQbEOl6	1	user073@aiarsenal.cc	1		r2ss4wkobo5w325niausqmror2ny4vpb		0				2026-07-04 00:00:00	2026-07-04 00:00:00	2026-07-04 00:00:00	\N	\N		0
75	hhqf1lr8ktqflwomy9ywlr3e2g87c1nr	user074	EMP00075	测试用户074	13910000074	测试用户074	0	$pbkdf2-sm3$60000$N7uxfKNz+g1dq7rgrDfrLQ$6/L5689KKo+8cZWusl0c/6BgJwnLTi2zPOzfo7SsWDw	cYfmbYAVZo	1	user074@aiarsenal.cc	2		lvyrsk0fjk7br36yh0ti1fug3c8oewsz		0				2026-07-04 00:00:00	2026-07-04 00:00:00	2026-07-04 00:00:00	\N	\N		0
76	vfuzzdizy3ladqg0f9bwb22csxl2b8zg	user075	EMP00076	测试用户075	13910000075	测试用户075	0	$pbkdf2-sm3$60000$0/PPexVTF94CTS1LY8ubrA$vMsUumpp6J+BGn/aTaZBC8wXcvMwUnrNSoEOfSLRjx8	RlKELGOPQI	1	user075@aiarsenal.cc	0		yvd2rzcq3p6on80czv6q7955j7gc5y3t		0				2026-07-04 00:00:00	2026-07-04 00:00:00	2026-07-04 00:00:00	\N	\N		0
77	bred3yjuutw1vt7mlwx4259tlez2hyvt	user076	EMP00077	测试用户076	13910000076	测试用户076	0	$pbkdf2-sm3$60000$I1HOOifoSFLZsD6HPAvwgw$idSyYSdsCnvEQlWJ0R9yjUfNCI3wK2VljMpvDq67Dqo	wS6wpaOU64	1	user076@aiarsenal.cc	1		oyy8hxpdfp1b5blfpq8or6tk15hvhwwk		0				2026-07-04 00:00:00	2026-07-04 00:00:00	2026-07-04 00:00:00	\N	\N		0
78	o109vwi25ey3ru4726gkeedt6yed15po	user077	EMP00078	测试用户077	13910000077	测试用户077	0	$pbkdf2-sm3$60000$r3zYO+NasnT1KlMT47FF4A$ch/+XXByAK35OoNByb8MLEQeQXX8pZvhgWd9C9LffS4	ZnpeIRpYBt	1	user077@aiarsenal.cc	2		g84a2esunq3ggigh6rie4iso5mvqejoc		0				2026-07-04 00:00:00	2026-07-04 00:00:00	2026-07-04 00:00:00	\N	\N		0
79	qpsr1il81pv2rpwsygkudq6ec28li8ee	user078	EMP00079	测试用户078	13910000078	测试用户078	0	$pbkdf2-sm3$60000$2lpjBXxXPsLz935/pU+j5Q$l82eQ2iM8uZFP649zaBMNUlOj8LqKvCpy1TKnij3Bbs	BLnUlau3X7	1	user078@aiarsenal.cc	0		nfnkkl3va2fdvuk9fwmiz21y15xpl8lh		0				2026-07-04 00:00:00	2026-07-04 00:00:00	2026-07-04 00:00:00	\N	\N		0
80	quguougzmiaoxn29p4jm7vfza0nu2mqn	user079	EMP00080	测试用户079	13910000079	测试用户079	0	$pbkdf2-sm3$60000$DGyKhQXY7lffhxTMoqyMyQ$+4TPBjVDVi9pZxR8tcX0+FOT9NE8304thfDktnimh3I	EOHBUzNT2A	1	user079@aiarsenal.cc	1		v5r1ilh98qyl0kwnsrfr79whbyy1twzf		0				2026-07-04 00:00:00	2026-07-04 00:00:00	2026-07-04 00:00:00	\N	\N		0
81	vbsw1wfxo4xiylrqp4ytlvb0iomct94u	user080	EMP00081	测试用户080	13910000080	测试用户080	0	$pbkdf2-sm3$60000$vrX6YIJ/Zkw5l5UfcSaWyA$ztz5q1WGpNCsTXGAbP35m+G0+ymL+8rBeR7ZcgcMnEw	2rMGWbq5Ei	1	user080@aiarsenal.cc	2		fnrcvzwdge0dwbwjaeyilydchrlmff9g		0				2026-07-04 00:00:00	2026-07-04 00:00:00	2026-07-04 00:00:00	\N	\N		0
82	rhdnuq6wo64neftkd68yny35yste2xtx	user081	EMP00082	测试用户081	13910000081	测试用户081	0	$pbkdf2-sm3$60000$wkzEdoX6RCGmJJJw7egZIA$0tMC0I8vqTNoKkhleFuV07bgA7PwaY9w91MXdiAyDZI	kjbvUSw4Or	1	user081@aiarsenal.cc	0		n7c0xt6ksdqwia6gjrtebatzlzz4c2tb		0				2026-07-04 00:00:00	2026-07-04 00:00:00	2026-07-04 00:00:00	\N	\N		0
83	p6m3p05a3n48hsaq294d431wybsq07zr	user082	EMP00083	测试用户082	13910000082	测试用户082	0	$pbkdf2-sm3$60000$QxA5r420B2i9Gwlj3UMyAw$zaP/5Bcbi8WHjICKl+6eQR2QLKEPx5cOF/Yq1yP2DhQ	4JD0e4I1WE	1	user082@aiarsenal.cc	1		qdei4bqdhl2s48229y716b7zmyl0yhov		0				2026-07-04 00:00:00	2026-07-04 00:00:00	2026-07-04 00:00:00	\N	\N		0
84	z5o2otfmasx8ylqa1iolk9z530n08opl	user083	EMP00084	测试用户083	13910000083	测试用户083	0	$pbkdf2-sm3$60000$KcrRCHd9g8jFnXeLG7Yzwg$DGVyEGmaj56i9SscCiWk9CUf6B+GUQaHL0HYt1/spz8	MmyPbmjIwy	1	user083@aiarsenal.cc	2		gzbhisqghr688sew022riq0m9ph1tnul		0				2026-07-04 00:00:00	2026-07-04 00:00:00	2026-07-04 00:00:00	\N	\N		0
85	fxufj8dsgmh7hwzc1zgbgoiptbr9smex	user084	EMP00085	测试用户084	13910000084	测试用户084	0	$pbkdf2-sm3$60000$cehkORu1FQwNC/UisMP0Nw$4TXZglzNEVl2uvP2lbj0IUxYN4v6csbRiQP6UcM7rzs	OMqA49es8s	1	user084@aiarsenal.cc	0		lcx6976nybie9legnbf7efda17gzms8p		0				2026-07-04 00:00:00	2026-07-04 00:00:00	2026-07-04 00:00:00	\N	\N		0
86	fyqkcyecpap2uqzaocs8q8t8x3cvmvfq	user085	EMP00086	测试用户085	13910000085	测试用户085	0	$pbkdf2-sm3$60000$OPN+1MnZfGYZyX30b6EuRA$v+4ng6qQrNTl/o8n93APUpt20+MlxeqXGk54buMf+Ss	tQfTelwEjT	1	user085@aiarsenal.cc	1		tuon2vwblsnov4mnykch7sypzbvwgqma		0				2026-07-04 00:00:00	2026-07-04 00:00:00	2026-07-04 00:00:00	\N	\N		0
87	v32y4lq3ljmqs3qgv8onxtk16gnutgri	user086	EMP00087	测试用户086	13910000086	测试用户086	0	$pbkdf2-sm3$60000$F5mW7jN5d+I0s2Q3aGl3EA$N0gkOc0+ri0YVd4az107GEAXVYbHRWQQbjQzBuF6gQ8	1sjUlpjIRn	1	user086@aiarsenal.cc	2		v82stxipz7d1bdq1iz4ovthbs019tphi		0				2026-07-04 00:00:00	2026-07-04 00:00:00	2026-07-04 00:00:00	\N	\N		0
88	y88icjiuzvqdnpjuh4lype9xt97a3xxr	user087	EMP00088	测试用户087	13910000087	测试用户087	0	$pbkdf2-sm3$60000$EuuJ+4yHyPx7JAWsichFPQ$cj8sHlXF6/B01H2q9S0YUTEi+hr4JXk7cjppIoDtxZ8	spc6M7e1BL	1	user087@aiarsenal.cc	0		kheqge89r0euopogj346fyg25m8t3rya		0				2026-07-04 00:00:00	2026-07-04 00:00:00	2026-07-04 00:00:00	\N	\N		0
89	smg1dhkia9fg7hjuycfk8449fz2dzj2a	user088	EMP00089	测试用户088	13910000088	测试用户088	0	$pbkdf2-sm3$60000$WmXsIRa0OHUOwR+tZHrsOA$caYJl4GeDTn+VF6f5kfEmFNsqUYJPGvoIOO8tWyu6DI	jpIVsdIafs	1	user088@aiarsenal.cc	1		fop8s0srp4a6or6ceam5nyp6632unzbg		0				2026-07-04 00:00:00	2026-07-04 00:00:00	2026-07-04 00:00:00	\N	\N		0
90	so85b77vby7plhnvr6pf17c98b9mf0to	user089	EMP00090	测试用户089	13910000089	测试用户089	0	$pbkdf2-sm3$60000$GQvTBFeIG8WT2d8gLw1ryw$xg5yZRBYF2ZLJgtCa6Cf0cUDEMvT0icLd4KVDa/V1lY	vnYXWthqUr	1	user089@aiarsenal.cc	2		hbxawf9h3d7xkyjmbw4uqeon4i3nfn2w		0				2026-07-04 00:00:00	2026-07-04 00:00:00	2026-07-04 00:00:00	\N	\N		0
91	r8144ojgas0uiprldnk21ckrbw6z9kgy	user090	EMP00091	测试用户090	13910000090	测试用户090	0	$pbkdf2-sm3$60000$rn70SkDsfpdo1yNMBCgNfg$PNmDBT/JOZ49HMA4whVNGdmG3yt2zir1F59fmhV3g2E	OMeMSCWoeG	1	user090@aiarsenal.cc	0		ivgstgmxznc1erf0sxvtxjqfikq7wazs		0				2026-07-04 00:00:00	2026-07-04 00:00:00	2026-07-04 00:00:00	\N	\N		0
92	tdunjhhjpzmmiw9eu8a9shulxqbfp8ec	user091	EMP00092	测试用户091	13910000091	测试用户091	0	$pbkdf2-sm3$60000$DU/gkFwtYXQYnhKyaSjfpQ$2ZDOnePWf+owRsdvSq/TInHBebSWSJTkT5ix+cxKDnA	oXT0B7utQO	1	user091@aiarsenal.cc	1		kye5pyfqoq5doipzenotptllr2g6mxqg		0				2026-07-04 00:00:00	2026-07-04 00:00:00	2026-07-04 00:00:00	\N	\N		0
93	mxlhyiou8vg6szccawm0zmpug4gg5xrb	user092	EMP00093	测试用户092	13910000092	测试用户092	0	$pbkdf2-sm3$60000$fNms+PjmfJBpI+HdtKJzhg$jO0r/ml/SaG1TatCAW2pbb6hPYqeExcELJghE7DOg0U	tFAGjPxjXv	1	user092@aiarsenal.cc	2		zujehiadtnjaqtgooixca2iqp9safihi		0				2026-07-04 00:00:00	2026-07-04 00:00:00	2026-07-04 00:00:00	\N	\N		0
94	nq2gkeoricp2ufl5ksvv6pznixkj7ifz	user093	EMP00094	测试用户093	13910000093	测试用户093	0	$pbkdf2-sm3$60000$q0tX1tsvenfnEd046DsT4w$XSl/yG8KOerAV8E3bCsOdWPeCIuy1SscYcnY33UF97c	wF1IZZLPJZ	1	user093@aiarsenal.cc	0		eow0qt4tkj5qw2k56wi30oupnb3yct4m		0				2026-07-04 00:00:00	2026-07-04 00:00:00	2026-07-04 00:00:00	\N	\N		0
95	hnvmzuzx6jup48fa2ow28va7vmjmipua	user094	EMP00095	测试用户094	13910000094	测试用户094	0	$pbkdf2-sm3$60000$jZiVkvbZ6N9M36bEPPJtZw$86hJnKgM1/VgQg9ka3gOALEKghzbqgtC9HjvmsfOTVA	HL2OZY2cJ7	1	user094@aiarsenal.cc	1		eleqfwc2744a9ozwn5ndpayuuw7c4zqy		0				2026-07-04 00:00:00	2026-07-04 00:00:00	2026-07-04 00:00:00	\N	\N		0
96	eieus50pzqqvzj1ge6l72zoina4v91gy	user095	EMP00096	测试用户095	13910000095	测试用户095	0	$pbkdf2-sm3$60000$10UYvEuJpWz37EbQo1ossg$pR20SM4NRa+cT/6LgEsq6bXR3d/5Wttf1BHmlUCnn4I	Q7la6HawOs	1	user095@aiarsenal.cc	2		llxllcao3wked14skgm1lb5i7p11a5jt		0				2026-07-04 00:00:00	2026-07-04 00:00:00	2026-07-04 00:00:00	\N	\N		0
97	rump0avc7t7htk5y52r61h7n1l90jv9f	user096	EMP00097	测试用户096	13910000096	测试用户096	0	$pbkdf2-sm3$60000$oTx+JokfCTXQy4slyp9GsQ$Zow3fsPljB5WVS6gYrxUkNnjfGFVRqmLmf1W00gECtc	flNVnMDWyi	1	user096@aiarsenal.cc	0		cyrj194puwursqce38vm471isql0qc4j		0				2026-07-04 00:00:00	2026-07-04 00:00:00	2026-07-04 00:00:00	\N	\N		0
98	fy7jo8iycp36dkl4rjko3oy6r8vknu3m	user097	EMP00098	测试用户097	13910000097	测试用户097	0	$pbkdf2-sm3$60000$TVp6ffaZeL6F27TPz7FEFg$s8Xy97ymxBkA/LYUdKA4oAVPW5azfv+TkPOsePMW1xY	9tZra6cQAy	1	user097@aiarsenal.cc	1		m2hu15ojpus1layz4k4zhifi9cj8erbt		0				2026-07-04 00:00:00	2026-07-04 00:00:00	2026-07-04 00:00:00	\N	\N		0
99	e2ctnmnw5esjmd5o5gp7zg2gw3f6mnwm	user098	EMP00099	测试用户098	13910000098	测试用户098	0	$pbkdf2-sm3$60000$pNVwpLhQ7xbnY38EpN5s3g$FsNHKW6KWxZJ4Nss5YfnAu18LwVcqyO9Im9qcsJfDGo	nFPMUFuf31	1	user098@aiarsenal.cc	2		b52vpvw2wompdleclbp5467pkw017oab		0				2026-07-04 00:00:00	2026-07-04 00:00:00	2026-07-04 00:00:00	\N	\N		0
100	ouzpq54s44roqs97m7ii1breztn12gtu	user099	EMP00100	测试用户099	13910000099	测试用户099	0	$pbkdf2-sm3$60000$BDTtBQtGBey/Bd5kNY4tzw$RgIGygrH1DvqnseDF1mK9b340VsKzPnuNXaoLsMxU50	gpevb7hLpi	1	user099@aiarsenal.cc	0		gb2wz5r8etns12cgiu4pi627n2sqqlfw		0				2026-07-04 00:00:00	2026-07-04 00:00:00	2026-07-04 00:00:00	\N	\N		0
101	s29fp7shxnshfy13z03uzw0rpnp9rkrm	user100	EMP00101	测试用户100	13910000100	测试用户100	0	$pbkdf2-sm3$60000$dzUCW7Y045WvPTMfTtbwKQ$irIxOFaf6UZ9s95GO2PWqP1B7jrtqwTX1i+DYotpjbc	zEkWDzoNwj	1	user100@aiarsenal.cc	1		tm1yja3iahb1f4v3bhc7q7amvcu9crhk		0				2026-07-04 00:00:00	2026-07-04 00:00:00	2026-07-04 00:00:00	\N	\N		0
1	wxd3a2drv1ikdkqxub4hp49e7vrvcd3t	hi	EMP00001	hi	13800000000	hi	0	$pbkdf2-sm3$60000$CixszrtreSVXgXuyOjVIxQ$edEas+axsjMfKGizCG5eO/lWlReeYEtzmTkkrDRGtM0	k7Qm2xP9aL	1	hi@aiarsenal.cc	1		dqr6r6e6exwggi02v2h0tqkbdd46c3sb		1				\N	2026-07-04 00:00:00	2026-07-04 00:00:00	\N	\N		0
\.

--
-- Data for Name: aa_user_post; Type: TABLE DATA; Schema: public; Owner: -
--
COPY public.aa_user_post (id, user_id, post_id) FROM stdin;
1	i47dhcwyx2rulbjl5vlab89orpamongb	jm8ebgikyyvpqdvxyq1koy3ka1hrkf3v
2	awaoolltywg611faqbrfjs0a6qq7e7mq	z9zwsqp1y5gklcyqwqywh2wbfg2ih0le
3	ttozve8tomu4bw7q64iqch473vic8xjx	las48k9l1ii0ir2962a5r4xfxame2btu
4	l7v7hxinitk68gtbwp1ygd1ud39q88bt	z9zwsqp1y5gklcyqwqywh2wbfg2ih0le
5	g1zzvvdxqrpmzk2kw8q4ky3a26e6qfxv	jm8ebgikyyvpqdvxyq1koy3ka1hrkf3v
6	wcsj3t9d38o5o5ryzp9pl0s72bbolqje	jm8ebgikyyvpqdvxyq1koy3ka1hrkf3v
7	psnpiaisc4utvtowod2cismxnen0sqji	p18peuyi9qaddojzcck35ufolgawzrpg
8	yftk7uwwhy2oxmrzkq3tp4dh89bch1jb	vetr6vywrxw1mfbhxco2oaaiz7okedet
9	ey81ndf6zpobmaoxqiquhofjpu02z8ht	z9zwsqp1y5gklcyqwqywh2wbfg2ih0le
10	hhhnxqvutljctsgaoo780vo563bez55a	vetr6vywrxw1mfbhxco2oaaiz7okedet
11	xdrjswzjeydx1v5ge05jltxd0ixg0yya	jm8ebgikyyvpqdvxyq1koy3ka1hrkf3v
12	qqzy7xff86q8kfmyopjq5dmgbwf03lkd	w1c2cofhzsnm559td9uot44p6mg35zza
13	hwogmz6ku42vzbxo7l4g9seiftxmf55z	las48k9l1ii0ir2962a5r4xfxame2btu
14	vxxrlq1k4nmwnbuiud4z8g4zdn6brnom	jm8ebgikyyvpqdvxyq1koy3ka1hrkf3v
15	t8ubxdsaiuixyj57rt57vhrggh6yejco	p18peuyi9qaddojzcck35ufolgawzrpg
16	huoc4rxc0p8g6f4cswatmogr6daqfxgl	z9zwsqp1y5gklcyqwqywh2wbfg2ih0le
17	yv6887f2ne6v8cwx0shm2ty97nbv4vme	vetr6vywrxw1mfbhxco2oaaiz7okedet
18	cz5dv5ogq1ifcqmsempf00hqx4w9e7fr	w1c2cofhzsnm559td9uot44p6mg35zza
19	ddv2o2ggneyz5omf2hsakq5gq5nb37kp	z9zwsqp1y5gklcyqwqywh2wbfg2ih0le
20	iozdijm6umoh07mwmf9shtyb3gkm6b2e	las48k9l1ii0ir2962a5r4xfxame2btu
21	xkbw9qo0volhjszdrgr11ieh9qmy05xm	z9zwsqp1y5gklcyqwqywh2wbfg2ih0le
22	ojmwz7cbldpxwnmu4h9ed5gzjm8yglbj	vetr6vywrxw1mfbhxco2oaaiz7okedet
23	ouvzpmuuf8vmoja8g7ua9x1a6m83dkqe	vetr6vywrxw1mfbhxco2oaaiz7okedet
24	f571jz7rptodss6igenpqkbdv9s8nhgq	jm8ebgikyyvpqdvxyq1koy3ka1hrkf3v
25	nn5sxc1va1wbi7c3y8nj3si1tznkil1o	vetr6vywrxw1mfbhxco2oaaiz7okedet
26	rjtjyaegy2r498hswx8htls1ncqz2shp	jm8ebgikyyvpqdvxyq1koy3ka1hrkf3v
27	o9ryfxtbs7mz7d1pitt1ji1eyofbuvhl	z9zwsqp1y5gklcyqwqywh2wbfg2ih0le
28	v8nyq4ekvn96ffe69fpteqprfbyw7d5o	z9zwsqp1y5gklcyqwqywh2wbfg2ih0le
29	ccp94mq5kpf7del1n8kixhght9hoc74f	p18peuyi9qaddojzcck35ufolgawzrpg
30	i5jq4vt4wk4pf3lb4nx2fh4339wbfz0y	vetr6vywrxw1mfbhxco2oaaiz7okedet
31	h30ls46rppecsk1di4n5zxlz7xpcps5i	vetr6vywrxw1mfbhxco2oaaiz7okedet
32	g4dp1tsgez5vvn1a2hq7vpgrireuwr3f	ec0minuxxdiktu5c3hqu72eayy06ljrz
33	xh9e2z238dalqjxse0gdlrteaf49325f	jm8ebgikyyvpqdvxyq1koy3ka1hrkf3v
34	p6k1ds5468pmzbz1i6v4814lt5475m3m	w1c2cofhzsnm559td9uot44p6mg35zza
35	nxbyzuyiu6ouyygcuzemmac873aljn3j	las48k9l1ii0ir2962a5r4xfxame2btu
36	f386t32hk8sghcnvf24g3st3i41phl9o	jm8ebgikyyvpqdvxyq1koy3ka1hrkf3v
37	qu9owum1ui0elni61b5i29akyl7jt03p	jm8ebgikyyvpqdvxyq1koy3ka1hrkf3v
38	b7qp8p0xj1okbz6vxrmzngd32jsonytp	p18peuyi9qaddojzcck35ufolgawzrpg
39	qh1et5ylq2fm1e82d32fl67gvnqo6riq	ec0minuxxdiktu5c3hqu72eayy06ljrz
40	tyrhx8gab3hzay0xmz65aia5j5xzjcuo	jm8ebgikyyvpqdvxyq1koy3ka1hrkf3v
41	wlkajsoyh27oscyja111i6bq5e5cd5gx	vetr6vywrxw1mfbhxco2oaaiz7okedet
42	ki2q6fgdt8o56u3vn2ckanvoy34qqq9a	vetr6vywrxw1mfbhxco2oaaiz7okedet
43	lsinkrec95cl8xk41pc1xa0nwp77r7nq	vetr6vywrxw1mfbhxco2oaaiz7okedet
44	u7keg12zsf1fp6wxnd0w78r03lk9p5ne	jm8ebgikyyvpqdvxyq1koy3ka1hrkf3v
45	vj9e202tchk0m7ar1pg1w2xvcb52770u	z9zwsqp1y5gklcyqwqywh2wbfg2ih0le
46	bdlps6p632yb96o44b00s23l1eywr0sy	z9zwsqp1y5gklcyqwqywh2wbfg2ih0le
47	fd5893szdczhr9ylo1cwbrsbb7f6s0np	ec0minuxxdiktu5c3hqu72eayy06ljrz
48	fisec94xg1zmsoe73xqi8s7pcioxs2xy	w1c2cofhzsnm559td9uot44p6mg35zza
49	qjnllgil3gdfowp1khpin6o2b6egz7vs	las48k9l1ii0ir2962a5r4xfxame2btu
50	et5k755mj9ndmzcz5uqj79o0se8a7hjq	jm8ebgikyyvpqdvxyq1koy3ka1hrkf3v
51	z1cnqvbthuj6j2ocm19p5wivwknkwwxb	vetr6vywrxw1mfbhxco2oaaiz7okedet
52	fgyo9wgof0kcalr2r8cskc2g0bejiw4r	ec0minuxxdiktu5c3hqu72eayy06ljrz
53	zzye39s02viahffg8futr2nq2m8ahw9v	p18peuyi9qaddojzcck35ufolgawzrpg
54	nwesstx94ue6ekr832lgdj7zz1r81yol	vetr6vywrxw1mfbhxco2oaaiz7okedet
55	fczzjf2lns76m9avq50s302n8bfv6kxy	z9zwsqp1y5gklcyqwqywh2wbfg2ih0le
56	ljkgwn4eowlvuh9aafzdc0ykhj06ptpw	w1c2cofhzsnm559td9uot44p6mg35zza
57	rbfrd3phrnqazpxz83q61q87t3s6znun	las48k9l1ii0ir2962a5r4xfxame2btu
58	qdroqjw2xx7ckth36lsw9lfmexyf1fbt	jm8ebgikyyvpqdvxyq1koy3ka1hrkf3v
59	nadp1hsoga0hqcw16pynzk19h7ehcj5r	jm8ebgikyyvpqdvxyq1koy3ka1hrkf3v
60	l5iu02zzxae7gw6denbs5m430pnakwco	jm8ebgikyyvpqdvxyq1koy3ka1hrkf3v
61	g5zzcwa6lljcra9bh95ot0wmdbo8bb1v	ec0minuxxdiktu5c3hqu72eayy06ljrz
62	idg18sc5jlnnbs02myk619oj5s8kstej	jm8ebgikyyvpqdvxyq1koy3ka1hrkf3v
63	ap2xvhdaf6feytr8bbfa0x563uuoztsm	ec0minuxxdiktu5c3hqu72eayy06ljrz
64	aqjsus7aopdpz778evymkbst0mwg6s4l	vetr6vywrxw1mfbhxco2oaaiz7okedet
65	y9i1zy0zq9jtjx57eleglzfvurltdf9v	p18peuyi9qaddojzcck35ufolgawzrpg
66	swy9yjech3it00fnpuk9m5t3ww50f0zu	vetr6vywrxw1mfbhxco2oaaiz7okedet
67	kp0b2zujdinw3quweijsa8k1fn5jo29l	jm8ebgikyyvpqdvxyq1koy3ka1hrkf3v
68	t6bh2x83k6hg6haogt4loyl3dw283d1d	vetr6vywrxw1mfbhxco2oaaiz7okedet
69	nk12a6f4keadmc12tfw69abqnb0hco5d	vetr6vywrxw1mfbhxco2oaaiz7okedet
70	zqb590opd1gatrzyadk6xax7e7bx500y	w1c2cofhzsnm559td9uot44p6mg35zza
71	hyztdi4uhb1w7ndfi47zpp1y5m16soai	las48k9l1ii0ir2962a5r4xfxame2btu
72	kb38c9j616dragwwgbyo8p7j4vk1xnub	ec0minuxxdiktu5c3hqu72eayy06ljrz
73	b0f6pm4d64m6zz6agzg7ff1y80kb112t	jm8ebgikyyvpqdvxyq1koy3ka1hrkf3v
74	hhqf1lr8ktqflwomy9ywlr3e2g87c1nr	jm8ebgikyyvpqdvxyq1koy3ka1hrkf3v
75	vfuzzdizy3ladqg0f9bwb22csxl2b8zg	vetr6vywrxw1mfbhxco2oaaiz7okedet
76	bred3yjuutw1vt7mlwx4259tlez2hyvt	vetr6vywrxw1mfbhxco2oaaiz7okedet
77	o109vwi25ey3ru4726gkeedt6yed15po	p18peuyi9qaddojzcck35ufolgawzrpg
78	qpsr1il81pv2rpwsygkudq6ec28li8ee	vetr6vywrxw1mfbhxco2oaaiz7okedet
79	quguougzmiaoxn29p4jm7vfza0nu2mqn	ec0minuxxdiktu5c3hqu72eayy06ljrz
80	vbsw1wfxo4xiylrqp4ytlvb0iomct94u	vetr6vywrxw1mfbhxco2oaaiz7okedet
81	rhdnuq6wo64neftkd68yny35yste2xtx	w1c2cofhzsnm559td9uot44p6mg35zza
82	p6m3p05a3n48hsaq294d431wybsq07zr	las48k9l1ii0ir2962a5r4xfxame2btu
83	z5o2otfmasx8ylqa1iolk9z530n08opl	jm8ebgikyyvpqdvxyq1koy3ka1hrkf3v
84	fxufj8dsgmh7hwzc1zgbgoiptbr9smex	ec0minuxxdiktu5c3hqu72eayy06ljrz
85	fyqkcyecpap2uqzaocs8q8t8x3cvmvfq	jm8ebgikyyvpqdvxyq1koy3ka1hrkf3v
86	v32y4lq3ljmqs3qgv8onxtk16gnutgri	jm8ebgikyyvpqdvxyq1koy3ka1hrkf3v
87	y88icjiuzvqdnpjuh4lype9xt97a3xxr	z9zwsqp1y5gklcyqwqywh2wbfg2ih0le
88	smg1dhkia9fg7hjuycfk8449fz2dzj2a	jm8ebgikyyvpqdvxyq1koy3ka1hrkf3v
89	so85b77vby7plhnvr6pf17c98b9mf0to	jm8ebgikyyvpqdvxyq1koy3ka1hrkf3v
90	r8144ojgas0uiprldnk21ckrbw6z9kgy	jm8ebgikyyvpqdvxyq1koy3ka1hrkf3v
91	tdunjhhjpzmmiw9eu8a9shulxqbfp8ec	ec0minuxxdiktu5c3hqu72eayy06ljrz
92	mxlhyiou8vg6szccawm0zmpug4gg5xrb	vetr6vywrxw1mfbhxco2oaaiz7okedet
93	nq2gkeoricp2ufl5ksvv6pznixkj7ifz	z9zwsqp1y5gklcyqwqywh2wbfg2ih0le
94	hnvmzuzx6jup48fa2ow28va7vmjmipua	z9zwsqp1y5gklcyqwqywh2wbfg2ih0le
95	eieus50pzqqvzj1ge6l72zoina4v91gy	jm8ebgikyyvpqdvxyq1koy3ka1hrkf3v
96	rump0avc7t7htk5y52r61h7n1l90jv9f	jm8ebgikyyvpqdvxyq1koy3ka1hrkf3v
97	fy7jo8iycp36dkl4rjko3oy6r8vknu3m	p18peuyi9qaddojzcck35ufolgawzrpg
98	e2ctnmnw5esjmd5o5gp7zg2gw3f6mnwm	vetr6vywrxw1mfbhxco2oaaiz7okedet
99	ouzpq54s44roqs97m7ii1breztn12gtu	z9zwsqp1y5gklcyqwqywh2wbfg2ih0le
100	s29fp7shxnshfy13z03uzw0rpnp9rkrm	w1c2cofhzsnm559td9uot44p6mg35zza
\.

COPY public.aa_rule (ptype, v0, v1, v2, v3, v4, v5) FROM stdin;
g	u_i47dhcwyx2rulbjl5vlab89orpamongb	h9driy19c07f6tgmxbpn1qp39vnqdv9c				
g	u_awaoolltywg611faqbrfjs0a6qq7e7mq	kt9c0598mbwnif8igl6ziaz55ehxqp8t				
g	u_ttozve8tomu4bw7q64iqch473vic8xjx	pql3anx0ft4qn31brg6b2ev2rpguptze				
g	u_l7v7hxinitk68gtbwp1ygd1ud39q88bt	kt9c0598mbwnif8igl6ziaz55ehxqp8t				
g	u_g1zzvvdxqrpmzk2kw8q4ky3a26e6qfxv	h9driy19c07f6tgmxbpn1qp39vnqdv9c				
g	u_wcsj3t9d38o5o5ryzp9pl0s72bbolqje	pql3anx0ft4qn31brg6b2ev2rpguptze				
g	u_psnpiaisc4utvtowod2cismxnen0sqji	pql3anx0ft4qn31brg6b2ev2rpguptze				
g	u_yftk7uwwhy2oxmrzkq3tp4dh89bch1jb	pql3anx0ft4qn31brg6b2ev2rpguptze				
g	u_ey81ndf6zpobmaoxqiquhofjpu02z8ht	kt9c0598mbwnif8igl6ziaz55ehxqp8t				
g	u_hhhnxqvutljctsgaoo780vo563bez55a	v8cw0kke6bb75tdk4oya61ku9zbi3dhz				
g	u_xdrjswzjeydx1v5ge05jltxd0ixg0yya	h9driy19c07f6tgmxbpn1qp39vnqdv9c				
g	u_qqzy7xff86q8kfmyopjq5dmgbwf03lkd	pql3anx0ft4qn31brg6b2ev2rpguptze				
g	u_hwogmz6ku42vzbxo7l4g9seiftxmf55z	pql3anx0ft4qn31brg6b2ev2rpguptze				
g	u_vxxrlq1k4nmwnbuiud4z8g4zdn6brnom	pql3anx0ft4qn31brg6b2ev2rpguptze				
g	u_t8ubxdsaiuixyj57rt57vhrggh6yejco	pql3anx0ft4qn31brg6b2ev2rpguptze				
g	u_huoc4rxc0p8g6f4cswatmogr6daqfxgl	kt9c0598mbwnif8igl6ziaz55ehxqp8t				
g	u_yv6887f2ne6v8cwx0shm2ty97nbv4vme	pql3anx0ft4qn31brg6b2ev2rpguptze				
g	u_cz5dv5ogq1ifcqmsempf00hqx4w9e7fr	pql3anx0ft4qn31brg6b2ev2rpguptze				
g	u_ddv2o2ggneyz5omf2hsakq5gq5nb37kp	kt9c0598mbwnif8igl6ziaz55ehxqp8t				
g	u_iozdijm6umoh07mwmf9shtyb3gkm6b2e	pql3anx0ft4qn31brg6b2ev2rpguptze				
g	u_xkbw9qo0volhjszdrgr11ieh9qmy05xm	kt9c0598mbwnif8igl6ziaz55ehxqp8t				
g	u_ojmwz7cbldpxwnmu4h9ed5gzjm8yglbj	v8cw0kke6bb75tdk4oya61ku9zbi3dhz				
g	u_ouvzpmuuf8vmoja8g7ua9x1a6m83dkqe	v8cw0kke6bb75tdk4oya61ku9zbi3dhz				
g	u_f571jz7rptodss6igenpqkbdv9s8nhgq	h9driy19c07f6tgmxbpn1qp39vnqdv9c				
g	u_nn5sxc1va1wbi7c3y8nj3si1tznkil1o	v8cw0kke6bb75tdk4oya61ku9zbi3dhz				
g	u_rjtjyaegy2r498hswx8htls1ncqz2shp	pql3anx0ft4qn31brg6b2ev2rpguptze				
g	u_o9ryfxtbs7mz7d1pitt1ji1eyofbuvhl	kt9c0598mbwnif8igl6ziaz55ehxqp8t				
g	u_v8nyq4ekvn96ffe69fpteqprfbyw7d5o	kt9c0598mbwnif8igl6ziaz55ehxqp8t				
g	u_ccp94mq5kpf7del1n8kixhght9hoc74f	pql3anx0ft4qn31brg6b2ev2rpguptze				
g	u_i5jq4vt4wk4pf3lb4nx2fh4339wbfz0y	pql3anx0ft4qn31brg6b2ev2rpguptze				
g	u_h30ls46rppecsk1di4n5zxlz7xpcps5i	v8cw0kke6bb75tdk4oya61ku9zbi3dhz				
g	u_g4dp1tsgez5vvn1a2hq7vpgrireuwr3f	yrbx49f8kvuilrnbn8zy4hc2k1x3h99u				
g	u_xh9e2z238dalqjxse0gdlrteaf49325f	h9driy19c07f6tgmxbpn1qp39vnqdv9c				
g	u_p6k1ds5468pmzbz1i6v4814lt5475m3m	pql3anx0ft4qn31brg6b2ev2rpguptze				
g	u_nxbyzuyiu6ouyygcuzemmac873aljn3j	pql3anx0ft4qn31brg6b2ev2rpguptze				
g	u_f386t32hk8sghcnvf24g3st3i41phl9o	h9driy19c07f6tgmxbpn1qp39vnqdv9c				
g	u_qu9owum1ui0elni61b5i29akyl7jt03p	pql3anx0ft4qn31brg6b2ev2rpguptze				
g	u_b7qp8p0xj1okbz6vxrmzngd32jsonytp	pql3anx0ft4qn31brg6b2ev2rpguptze				
g	u_qh1et5ylq2fm1e82d32fl67gvnqo6riq	yrbx49f8kvuilrnbn8zy4hc2k1x3h99u				
g	u_tyrhx8gab3hzay0xmz65aia5j5xzjcuo	h9driy19c07f6tgmxbpn1qp39vnqdv9c				
g	u_wlkajsoyh27oscyja111i6bq5e5cd5gx	v8cw0kke6bb75tdk4oya61ku9zbi3dhz				
g	u_ki2q6fgdt8o56u3vn2ckanvoy34qqq9a	pql3anx0ft4qn31brg6b2ev2rpguptze				
g	u_lsinkrec95cl8xk41pc1xa0nwp77r7nq	v8cw0kke6bb75tdk4oya61ku9zbi3dhz				
g	u_u7keg12zsf1fp6wxnd0w78r03lk9p5ne	h9driy19c07f6tgmxbpn1qp39vnqdv9c				
g	u_vj9e202tchk0m7ar1pg1w2xvcb52770u	kt9c0598mbwnif8igl6ziaz55ehxqp8t				
g	u_bdlps6p632yb96o44b00s23l1eywr0sy	kt9c0598mbwnif8igl6ziaz55ehxqp8t				
g	u_fd5893szdczhr9ylo1cwbrsbb7f6s0np	yrbx49f8kvuilrnbn8zy4hc2k1x3h99u				
g	u_fisec94xg1zmsoe73xqi8s7pcioxs2xy	pql3anx0ft4qn31brg6b2ev2rpguptze				
g	u_qjnllgil3gdfowp1khpin6o2b6egz7vs	pql3anx0ft4qn31brg6b2ev2rpguptze				
g	u_et5k755mj9ndmzcz5uqj79o0se8a7hjq	pql3anx0ft4qn31brg6b2ev2rpguptze				
g	u_z1cnqvbthuj6j2ocm19p5wivwknkwwxb	v8cw0kke6bb75tdk4oya61ku9zbi3dhz				
g	u_fgyo9wgof0kcalr2r8cskc2g0bejiw4r	yrbx49f8kvuilrnbn8zy4hc2k1x3h99u				
g	u_zzye39s02viahffg8futr2nq2m8ahw9v	pql3anx0ft4qn31brg6b2ev2rpguptze				
g	u_nwesstx94ue6ekr832lgdj7zz1r81yol	pql3anx0ft4qn31brg6b2ev2rpguptze				
g	u_fczzjf2lns76m9avq50s302n8bfv6kxy	kt9c0598mbwnif8igl6ziaz55ehxqp8t				
g	u_ljkgwn4eowlvuh9aafzdc0ykhj06ptpw	pql3anx0ft4qn31brg6b2ev2rpguptze				
g	u_rbfrd3phrnqazpxz83q61q87t3s6znun	pql3anx0ft4qn31brg6b2ev2rpguptze				
g	u_qdroqjw2xx7ckth36lsw9lfmexyf1fbt	pql3anx0ft4qn31brg6b2ev2rpguptze				
g	u_nadp1hsoga0hqcw16pynzk19h7ehcj5r	h9driy19c07f6tgmxbpn1qp39vnqdv9c				
g	u_l5iu02zzxae7gw6denbs5m430pnakwco	h9driy19c07f6tgmxbpn1qp39vnqdv9c				
g	u_g5zzcwa6lljcra9bh95ot0wmdbo8bb1v	yrbx49f8kvuilrnbn8zy4hc2k1x3h99u				
g	u_idg18sc5jlnnbs02myk619oj5s8kstej	h9driy19c07f6tgmxbpn1qp39vnqdv9c				
g	u_ap2xvhdaf6feytr8bbfa0x563uuoztsm	yrbx49f8kvuilrnbn8zy4hc2k1x3h99u				
g	u_aqjsus7aopdpz778evymkbst0mwg6s4l	v8cw0kke6bb75tdk4oya61ku9zbi3dhz				
g	u_y9i1zy0zq9jtjx57eleglzfvurltdf9v	pql3anx0ft4qn31brg6b2ev2rpguptze				
g	u_swy9yjech3it00fnpuk9m5t3ww50f0zu	pql3anx0ft4qn31brg6b2ev2rpguptze				
g	u_kp0b2zujdinw3quweijsa8k1fn5jo29l	h9driy19c07f6tgmxbpn1qp39vnqdv9c				
g	u_t6bh2x83k6hg6haogt4loyl3dw283d1d	v8cw0kke6bb75tdk4oya61ku9zbi3dhz				
g	u_nk12a6f4keadmc12tfw69abqnb0hco5d	v8cw0kke6bb75tdk4oya61ku9zbi3dhz				
g	u_zqb590opd1gatrzyadk6xax7e7bx500y	pql3anx0ft4qn31brg6b2ev2rpguptze				
g	u_hyztdi4uhb1w7ndfi47zpp1y5m16soai	pql3anx0ft4qn31brg6b2ev2rpguptze				
g	u_kb38c9j616dragwwgbyo8p7j4vk1xnub	yrbx49f8kvuilrnbn8zy4hc2k1x3h99u				
g	u_b0f6pm4d64m6zz6agzg7ff1y80kb112t	h9driy19c07f6tgmxbpn1qp39vnqdv9c				
g	u_hhqf1lr8ktqflwomy9ywlr3e2g87c1nr	pql3anx0ft4qn31brg6b2ev2rpguptze				
g	u_vfuzzdizy3ladqg0f9bwb22csxl2b8zg	v8cw0kke6bb75tdk4oya61ku9zbi3dhz				
g	u_bred3yjuutw1vt7mlwx4259tlez2hyvt	v8cw0kke6bb75tdk4oya61ku9zbi3dhz				
g	u_o109vwi25ey3ru4726gkeedt6yed15po	pql3anx0ft4qn31brg6b2ev2rpguptze				
g	u_qpsr1il81pv2rpwsygkudq6ec28li8ee	pql3anx0ft4qn31brg6b2ev2rpguptze				
g	u_quguougzmiaoxn29p4jm7vfza0nu2mqn	yrbx49f8kvuilrnbn8zy4hc2k1x3h99u				
g	u_vbsw1wfxo4xiylrqp4ytlvb0iomct94u	v8cw0kke6bb75tdk4oya61ku9zbi3dhz				
g	u_rhdnuq6wo64neftkd68yny35yste2xtx	pql3anx0ft4qn31brg6b2ev2rpguptze				
g	u_p6m3p05a3n48hsaq294d431wybsq07zr	pql3anx0ft4qn31brg6b2ev2rpguptze				
g	u_z5o2otfmasx8ylqa1iolk9z530n08opl	h9driy19c07f6tgmxbpn1qp39vnqdv9c				
g	u_fxufj8dsgmh7hwzc1zgbgoiptbr9smex	yrbx49f8kvuilrnbn8zy4hc2k1x3h99u				
g	u_fyqkcyecpap2uqzaocs8q8t8x3cvmvfq	h9driy19c07f6tgmxbpn1qp39vnqdv9c				
g	u_v32y4lq3ljmqs3qgv8onxtk16gnutgri	h9driy19c07f6tgmxbpn1qp39vnqdv9c				
g	u_y88icjiuzvqdnpjuh4lype9xt97a3xxr	kt9c0598mbwnif8igl6ziaz55ehxqp8t				
g	u_smg1dhkia9fg7hjuycfk8449fz2dzj2a	h9driy19c07f6tgmxbpn1qp39vnqdv9c				
g	u_so85b77vby7plhnvr6pf17c98b9mf0to	h9driy19c07f6tgmxbpn1qp39vnqdv9c				
g	u_r8144ojgas0uiprldnk21ckrbw6z9kgy	h9driy19c07f6tgmxbpn1qp39vnqdv9c				
g	u_tdunjhhjpzmmiw9eu8a9shulxqbfp8ec	yrbx49f8kvuilrnbn8zy4hc2k1x3h99u				
g	u_mxlhyiou8vg6szccawm0zmpug4gg5xrb	v8cw0kke6bb75tdk4oya61ku9zbi3dhz				
g	u_nq2gkeoricp2ufl5ksvv6pznixkj7ifz	kt9c0598mbwnif8igl6ziaz55ehxqp8t				
g	u_hnvmzuzx6jup48fa2ow28va7vmjmipua	kt9c0598mbwnif8igl6ziaz55ehxqp8t				
g	u_eieus50pzqqvzj1ge6l72zoina4v91gy	h9driy19c07f6tgmxbpn1qp39vnqdv9c				
g	u_rump0avc7t7htk5y52r61h7n1l90jv9f	pql3anx0ft4qn31brg6b2ev2rpguptze				
g	u_fy7jo8iycp36dkl4rjko3oy6r8vknu3m	pql3anx0ft4qn31brg6b2ev2rpguptze				
g	u_e2ctnmnw5esjmd5o5gp7zg2gw3f6mnwm	pql3anx0ft4qn31brg6b2ev2rpguptze				
g	u_ouzpq54s44roqs97m7ii1breztn12gtu	kt9c0598mbwnif8igl6ziaz55ehxqp8t				
g	u_s29fp7shxnshfy13z03uzw0rpnp9rkrm	pql3anx0ft4qn31brg6b2ev2rpguptze				
\.
