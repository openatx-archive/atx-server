# coding: utf-8
#
'''
用途：更新ATX-Server上的覆盖率数据（来源umeng）

{
    "equipment": {
        "item22": {
            "android": {
                "2017-10": {
                    "brandRankData": [
                        {
                            "name": "OPPO",
                            "value": 18.57,
                            "children": [
                                {
                                    "name": "OPPO R9",
                                    "value": 2.15
                                }
                            ]
                        },
                        {
                            "name": "vivo",
                            "value": 16.84,
                            "children": []
                        }
                    ],
                    "phoneTypeData": [
                        {
                            "name": "OPPO R9",
                            "value": 2.15,
                            "trend": "up"
                        }
                    ]
                },
                "2017-09": {},
                "2017-08": {},
                "2017-07": {},
                "2017-06": {},
                "2017-05": {},
                "2017-04": {},
                "2017-03": {}
            },
            "ios": {
                "2017-10": {},
                "2017-09": {},
                "2017-08": {},
                "2017-07": {},
                "2017-06": {},
                "2017-05": {},
                "2017-04": {},
                "2017-03": {}
            }
        }
    }
}

'''
import requests
import json
import levenshtein


def get_umeng_data():
    umeng_data = requests.get('http://compass.umeng.com/data/equipmentItem2_2.json').json()
    android_datas = umeng_data['equipment']['item22']['android']
    keys = sorted(android_datas.keys(), reverse=True)
    key = keys[0]
    print("Year-month:", key)
    rank_data = android_datas[key]['brandRankData']
    data = {}

    for brand in rank_data:
        for cov in brand['children']:
            name, value = cov['name'], cov['value']
            if name.startswith('畅玩'):
                name = '荣耀'+name
            if name.startswith('畅享'):
                name = '华为'+name
            data[name] = value
    
    rank_data = android_datas[keys[1]]['brandRankData']
    for brand in rank_data:
        for cov in brand['children']:
            name, value = cov['name'], cov['value']
            if name.startswith('畅玩'):
                name = '荣耀'+name
            if name.startswith('畅享'):
                name = '华为'+name
            if name not in data:
                data[name] = value
    return data


def main():
    data = get_umeng_data()
    keys = data.keys()

    for device in requests.get('http://10.246.46.160:8200/list').json():
        product = device.get('product') or {}
        name = product.get('name').lower()
        if not name:
            continue
        
        udid = device.get('udid')

        # 查找最匹配的名字
        mindist = 1e9
        bestkey = ''
        for key in keys:
            dist = levenshtein.match_distance(name, key.lower())
            if mindist > dist:
                mindist = dist
                bestkey = key
        
        # 全部匹配自动更新
        if mindist == 0:
            print(name, "==", bestkey, ">>", mindist, data[bestkey])
            requests.put('http://10.246.46.160:8200/devices/'+udid+'/product', data=json.dumps({
                'id': product['id'], 
                'coverage': data[bestkey]
            }))
        else:
            print("?", name, "==", bestkey, ">>", mindist, data[bestkey])
            confirm = input("Confirm update [Y/n]")
            if confirm == '':
                # print('update name')
                # requests.post('http://10.246.46.160:8200/devices/'+udid+'/info', data=json.dumps({'name': bestkey}))
                print('update coverage')
                requests.put('http://10.246.46.160:8200/devices/'+udid+'/product', data=json.dumps({
                    'id': product['id'], 
                    'name': bestkey,
                    'coverage': data[bestkey]
                }))


if __name__ == '__main__':
    main()
    
